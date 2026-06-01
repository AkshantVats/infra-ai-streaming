//! Async Kafka producer: WAL entries are acked only after broker delivery confirmation.

use std::sync::Arc;
use std::time::Duration;

use anyhow::Context;
use bytes::Bytes;
use rdkafka::config::ClientConfig;
use rdkafka::message::{Header, OwnedHeaders};
use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::util::Timeout;
use tokio::sync::Mutex;

use crate::metrics;
use crate::wal::WalWriter;

const PRODUCE_TIMEOUT: Duration = Duration::from_secs(5);
const ERROR_TYPE_MAX_RETRIES: &str = "max_retries";

/// Outbound Kafka record tied to a WAL line (acked after delivery).
#[derive(Debug, Clone)]
pub struct ProduceMessage {
    pub batch_id: String,
    /// Tenant id — Kafka partition key for per-tenant ordering.
    pub partition_key: String,
    pub payload: Bytes,
    pub wal_entry_id: u64,
}

/// Idempotent LZ4-compressed producer with primary topic + DLQ fallback.
pub struct KafkaProducer {
    producer: FutureProducer,
    topic: String,
    dlq_topic: String,
}

impl KafkaProducer {
    /// Build a `FutureProducer` with the ingestion durability / throughput settings from DESIGN.md.
    pub fn new(brokers: &str, topic: &str, dlq_topic: &str) -> anyhow::Result<Self> {
        let producer = producer_client_config(brokers)
            .create()
            .context("create kafka FutureProducer")?;
        Ok(Self {
            producer,
            topic: topic.to_string(),
            dlq_topic: dlq_topic.to_string(),
        })
    }

    /// Produce to the primary topic; on confirmed delivery, mark the WAL entry acked.
    ///
    /// After librdkafka exhausts retries, the payload is sent to the DLQ topic, metrics are
    /// incremented, and the error is returned (WAL remains unacked for replay).
    pub async fn produce(
        &self,
        msg: ProduceMessage,
        wal: Arc<Mutex<WalWriter>>,
    ) -> anyhow::Result<()> {
        match self
            .send_to_topic(&self.topic, &msg.partition_key, &msg.payload, &msg.batch_id)
            .await
        {
            Ok(()) => {
                wal.lock()
                    .await
                    .mark_acked(msg.wal_entry_id)
                    .with_context(|| format!("mark_acked wal entry {}", msg.wal_entry_id))?;
                Ok(())
            }
            Err(e) => self.handle_produce_failure(&msg, e).await,
        }
    }

    async fn send_to_topic(
        &self,
        topic: &str,
        partition_key: &str,
        payload: &Bytes,
        batch_id: &str,
    ) -> anyhow::Result<()> {
        let record = FutureRecord::to(topic)
            .key(partition_key)
            .payload(payload.as_ref())
            .headers(batch_id_headers(batch_id));

        self.producer
            .send(record, Timeout::After(PRODUCE_TIMEOUT))
            .await
            .map_err(|(err, _owned)| err)
            .context("kafka delivery")?;

        Ok(())
    }

    async fn handle_produce_failure(
        &self,
        msg: &ProduceMessage,
        err: anyhow::Error,
    ) -> anyhow::Result<()> {
        let dlq_result = self
            .send_to_topic(
                &self.dlq_topic,
                &msg.partition_key,
                &msg.payload,
                &msg.batch_id,
            )
            .await;

        metrics::KAFKA_PRODUCE_ERRORS_TOTAL
            .with_label_values(&[msg.partition_key.as_str(), ERROR_TYPE_MAX_RETRIES])
            .inc();

        tracing::error!(
            batch_id = %msg.batch_id,
            error = ?err,
            dlq_ok = dlq_result.is_ok(),
            "Kafka produce failed, sent to DLQ"
        );

        if let Err(dlq_err) = dlq_result {
            return Err(err
                .context(dlq_err)
                .context("kafka produce failed; DLQ produce also failed"));
        }

        Err(err.context("kafka produce failed after retries; payload sent to DLQ"))
    }
}

/// Shared producer `ClientConfig` (unit-tested without a live broker).
pub(crate) fn producer_client_config(brokers: &str) -> ClientConfig {
    let mut config = ClientConfig::new();
    config
        .set("bootstrap.servers", brokers)
        .set("message.timeout.ms", "5000")
        .set("enable.idempotence", "true")
        .set("acks", "all")
        .set("compression.type", "lz4")
        .set("batch.size", "1048576")
        .set("linger.ms", "5")
        .set("retries", "3");
    config
}

fn batch_id_headers(batch_id: &str) -> OwnedHeaders {
    OwnedHeaders::new().insert(Header {
        key: "batch_id",
        value: Some(batch_id.as_bytes()),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn producer_client_config_sets_expected_options() {
        let cfg = producer_client_config("broker1:9092,broker2:9092");
        assert_eq!(
            cfg.get("bootstrap.servers"),
            Some("broker1:9092,broker2:9092")
        );
        assert_eq!(cfg.get("message.timeout.ms"), Some("5000"));
        assert_eq!(cfg.get("enable.idempotence"), Some("true"));
        assert_eq!(cfg.get("acks"), Some("all"));
        assert_eq!(cfg.get("compression.type"), Some("lz4"));
        assert_eq!(cfg.get("batch.size"), Some("1048576"));
        assert_eq!(cfg.get("linger.ms"), Some("5"));
        assert_eq!(cfg.get("retries"), Some("3"));
    }

    #[test]
    fn produce_message_holds_wal_entry_id() {
        let msg = ProduceMessage {
            batch_id: "batch-1".into(),
            partition_key: "tenant-a".into(),
            payload: Bytes::from_static(b"{}"),
            wal_entry_id: 42,
        };
        assert_eq!(msg.wal_entry_id, 42);
        assert_eq!(msg.partition_key, "tenant-a");
    }

    #[tokio::test]
    async fn mark_acked_after_success_path_without_kafka() {
        let dir = tempfile::tempdir().expect("tempdir");
        let base = dir.path().to_str().expect("utf8 path");
        let mut wal = WalWriter::new(base).expect("wal");
        let entry_id = wal.append("b1", r#"{"events":[]}"#).expect("append");
        let wal = Arc::new(Mutex::new(wal));

        wal.lock()
            .await
            .mark_acked(entry_id)
            .expect("mark_acked mirrors post-delivery ack");
        let unacked = wal.lock().await.replay_unacked().expect("replay");
        assert!(unacked.is_empty());
    }
}
