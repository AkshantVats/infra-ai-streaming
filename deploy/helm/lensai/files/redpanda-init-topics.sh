#!/bin/sh
# Idempotent topic creation for local dev (8 partitions; 32 in production per DESIGN.md).
set -e

BROKERS="${KAFKA_BROKERS:-redpanda:9092}"
PARTITIONS="${KAFKA_TOPIC_PARTITIONS:-8}"

echo "Creating topics on ${BROKERS} (${PARTITIONS} partitions each)..."

for topic in "${KAFKA_TOPIC:-ai_inference_events}" "${KAFKA_DLQ_TOPIC:-ai_inference_dlq}" "${KAFKA_ANOMALIES_TOPIC:-ai_anomalies}"; do
  rpk topic create "${topic}" --brokers "${BROKERS}" -p "${PARTITIONS}" || true
done

echo "Topics ready:"
rpk topic list --brokers "${BROKERS}"
