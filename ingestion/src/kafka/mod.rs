//! Kafka producer for the ingest hot path (delivery-confirmed WAL ack).

pub mod producer;

pub use producer::{KafkaProducer, ProduceMessage};
