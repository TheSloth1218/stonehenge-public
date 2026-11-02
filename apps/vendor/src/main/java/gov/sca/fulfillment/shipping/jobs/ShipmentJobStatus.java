package gov.sca.fulfillment.shipping.jobs;

public enum ShipmentJobStatus {
    QUEUED,
    CLAIMED,
    PROCESSING,
    COMPLETED,
    RETRYABLE_FAILED,
    PERMANENTLY_FAILED
}
