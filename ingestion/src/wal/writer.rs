//! Segment WAL: newline JSON `WalEntry` per line, fsync after each append.

use std::fs::{self, File, OpenOptions};
use std::io::{BufRead, BufReader, BufWriter, Write};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use anyhow::Context;
use serde::{Deserialize, Serialize};

use crate::metrics;

/// One persisted batch line in the WAL.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct WalEntry {
    pub entry_id: u64,
    pub batch_id: String,
    pub events_json: String,
    pub written_at_unix_ms: u64,
    pub kafka_acked: bool,
}

const DEFAULT_MAX_SEGMENT_BYTES: usize = 67_108_864; // 64 MiB

fn segment_name(id: u64) -> String {
    format!("segment_{:010}.wal", id)
}

fn parse_segment_id(name: &str) -> Option<u64> {
    let rest = name.strip_prefix("segment_")?.strip_suffix(".wal")?;
    rest.parse().ok()
}

fn list_segment_ids(base_dir: &Path) -> anyhow::Result<Vec<u64>> {
    let mut ids = Vec::new();
    if !base_dir.exists() {
        return Ok(ids);
    }
    for ent in fs::read_dir(base_dir).with_context(|| format!("read_dir {}", base_dir.display()))? {
        let ent = ent.context("dir entry")?;
        let name = ent.file_name().to_string_lossy().into_owned();
        if let Some(id) = parse_segment_id(&name) {
            ids.push(id);
        }
    }
    ids.sort_unstable();
    Ok(ids)
}

fn ack_path(base_dir: &Path, entry_id: u64) -> PathBuf {
    base_dir.join("acks").join(format!("{entry_id}.ack"))
}

fn max_entry_id_across_segments(base_dir: &Path) -> anyhow::Result<u64> {
    let mut max_id = 0_u64;
    for id in list_segment_ids(base_dir)? {
        let path = base_dir.join(segment_name(id));
        let file = File::open(&path).with_context(|| format!("open {}", path.display()))?;
        for line in BufReader::new(file).lines() {
            let line = line.context("read wal line")?;
            if line.trim().is_empty() {
                continue;
            }
            match serde_json::from_str::<WalEntry>(&line) {
                Ok(e) => max_id = max_id.max(e.entry_id),
                Err(err) => {
                    tracing::warn!(path = %path.display(), %err, "skipping corrupt wal line");
                }
            }
        }
    }
    Ok(max_id)
}

fn segments_with_unacked_count(base_dir: &Path) -> anyhow::Result<i64> {
    let mut count = 0_i64;
    for seg_id in list_segment_ids(base_dir)? {
        let path = base_dir.join(segment_name(seg_id));
        let file = File::open(&path).with_context(|| format!("open {}", path.display()))?;
        let mut any_unacked = false;
        for line in BufReader::new(file).lines() {
            let line = line.context("read wal line")?;
            if line.trim().is_empty() {
                continue;
            }
            let entry: WalEntry = match serde_json::from_str(&line) {
                Ok(e) => e,
                Err(_) => continue,
            };
            if !ack_path(base_dir, entry.entry_id).exists() {
                any_unacked = true;
                break;
            }
        }
        if any_unacked {
            count += 1;
        }
    }
    Ok(count)
}

fn sync_buf(writer: &mut BufWriter<File>) -> anyhow::Result<()> {
    writer.flush().context("wal buf flush")?;
    writer
        .get_mut()
        .sync_all()
        .context("wal file fsync")?;
    Ok(())
}

/// Append-only WAL with per-entry ack sidecar files.
pub struct WalWriter {
    base_dir: PathBuf,
    current_file: BufWriter<File>,
    current_segment_id: u64,
    current_bytes: usize,
    max_segment_bytes: usize,
    next_entry_id: AtomicU64,
}

impl WalWriter {
    /// Open or create WAL under `base_dir`.
    pub fn new(base_dir: &str) -> anyhow::Result<Self> {
        let base_path = PathBuf::from(base_dir);
        fs::create_dir_all(&base_path).with_context(|| format!("create_dir_all {}", base_path.display()))?;
        fs::create_dir_all(base_path.join("acks"))
            .with_context(|| format!("create acks under {}", base_path.display()))?;

        let segment_ids = list_segment_ids(&base_path)?;
        let (current_segment_id, file, current_bytes) = if segment_ids.is_empty() {
            let id = 1_u64;
            let path = base_path.join(segment_name(id));
            let f = OpenOptions::new()
                .create(true)
                .append(true)
                .open(&path)
                .with_context(|| format!("create segment {}", path.display()))?;
            (id, f, 0_usize)
        } else {
            let id = *segment_ids.last().expect("non-empty");
            let path = base_path.join(segment_name(id));
            let f = OpenOptions::new()
                .append(true)
                .open(&path)
                .with_context(|| format!("append segment {}", path.display()))?;
            let len = f.metadata().context("segment metadata")?.len() as usize;
            (id, f, len)
        };

        let max_entry = max_entry_id_across_segments(&base_path)?;
        let next = max_entry.saturating_add(1);
        let pending = segments_with_unacked_count(&base_path).unwrap_or(0);
        metrics::WAL_SEGMENTS_PENDING.set(pending);

        Ok(Self {
            base_dir: base_path,
            current_file: BufWriter::new(file),
            current_segment_id,
            current_bytes,
            max_segment_bytes: DEFAULT_MAX_SEGMENT_BYTES,
            next_entry_id: AtomicU64::new(next),
        })
    }

    fn rotate(&mut self) -> anyhow::Result<()> {
        sync_buf(&mut self.current_file).context("wal rotate sync")?;
        self.current_segment_id = self.current_segment_id.saturating_add(1);
        let path = self.base_dir.join(segment_name(self.current_segment_id));
        let f = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&path)
            .with_context(|| format!("rotate open {}", path.display()))?;
        self.current_file = BufWriter::new(f);
        self.current_bytes = 0;
        self.refresh_segments_pending()
            .context("wal_segments_pending after rotate")?;
        Ok(())
    }

    fn refresh_segments_pending(&self) -> anyhow::Result<()> {
        let n = segments_with_unacked_count(&self.base_dir)?;
        metrics::WAL_SEGMENTS_PENDING.set(n);
        Ok(())
    }

    /// Append one batch line; returns monotonic `entry_id`. Fsync before return.
    pub fn append(&mut self, batch_id: &str, events_json: &str) -> anyhow::Result<u64> {
        let entry_id = self.next_entry_id.fetch_add(1, Ordering::SeqCst);
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .context("unix epoch")?
            .as_millis() as u64;
        let entry = WalEntry {
            entry_id,
            batch_id: batch_id.to_string(),
            events_json: events_json.to_string(),
            written_at_unix_ms: now,
            kafka_acked: false,
        };
        let line = serde_json::to_string(&entry).context("serialize WalEntry")?;
        let line_with_nl = format!("{line}\n");
        let line_bytes = line_with_nl.as_bytes().len();

        if self.current_bytes.saturating_add(line_bytes) > self.max_segment_bytes {
            self.rotate().context("wal segment rotate")?;
        }

        self.current_file
            .write_all(line_with_nl.as_bytes())
            .context("wal write line")?;
        self.current_bytes = self.current_bytes.saturating_add(line_bytes);
        sync_buf(&mut self.current_file).context("wal append fsync")?;

        self.refresh_segments_pending()
            .context("wal_segments_pending after append")?;
        Ok(entry_id)
    }

    /// Mark `entry_id` as Kafka-acked (survives replay).
    pub fn mark_acked(&self, entry_id: u64) -> anyhow::Result<()> {
        let dir = self.base_dir.join("acks");
        fs::create_dir_all(&dir).context("create acks dir")?;
        let path = ack_path(&self.base_dir, entry_id);
        let mut f = OpenOptions::new()
            .create(true)
            .write(true)
            .truncate(true)
            .open(&path)
            .with_context(|| format!("write ack {}", path.display()))?;
        f.sync_all().context("ack fsync")?;
        self.refresh_segments_pending()
            .context("wal_segments_pending after mark_acked")?;
        Ok(())
    }

    /// Entries not covered by an ack file (for Kafka replay on startup).
    pub fn replay_unacked(&self) -> anyhow::Result<Vec<WalEntry>> {
        let mut out = Vec::new();
        for seg_id in list_segment_ids(&self.base_dir)? {
            let path = self.base_dir.join(segment_name(seg_id));
            let file = File::open(&path).with_context(|| format!("replay open {}", path.display()))?;
            for line in BufReader::new(file).lines() {
                let line = line.context("replay read line")?;
                if line.trim().is_empty() {
                    continue;
                }
                let entry: WalEntry = match serde_json::from_str(&line) {
                    Ok(e) => e,
                    Err(err) => {
                        tracing::warn!(path = %path.display(), %err, "replay skipping corrupt line");
                        continue;
                    }
                };
                if ack_path(&self.base_dir, entry.entry_id).exists() {
                    continue;
                }
                metrics::WAL_REPLAY_EVENTS_TOTAL.inc();
                out.push(entry);
            }
        }
        self.refresh_segments_pending()
            .context("wal_segments_pending after replay")?;
        Ok(out)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::tempdir;

    #[test]
    fn append_mark_acked_replay() {
        let dir = tempdir().expect("tempdir");
        let base = dir.path().to_str().expect("utf8");
        let mut w = WalWriter::new(base).expect("new wal");
        let id = w.append("b1", r#"{"events":[]}"#).expect("append");
        assert_eq!(id, 1);
        let unacked = w.replay_unacked().expect("replay");
        assert_eq!(unacked.len(), 1);
        assert_eq!(unacked[0].entry_id, 1);

        w.mark_acked(1).expect("ack");
        let unacked2 = w.replay_unacked().expect("replay2");
        assert!(unacked2.is_empty());
    }

    #[test]
    fn append_increments_entry_id() {
        let dir = tempdir().expect("tempdir");
        let base = dir.path().to_str().expect("utf8");
        let mut w = WalWriter::new(base).expect("new wal");
        assert_eq!(w.append("a", "{}").unwrap(), 1);
        assert_eq!(w.append("b", "{}").unwrap(), 2);
    }
}
