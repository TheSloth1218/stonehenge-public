package gov.sca.fulfillment.fulfillment;

public enum OrderStatus {
    DRAFT,
    PICKING,
    REVIEW,
    READY_TO_SHIP,
    SHIPPED,
    COMPLETION_PENDING,
    COMPLETION_SENT,
    COMPLETION_FAILED,
    CANCELLED
}
