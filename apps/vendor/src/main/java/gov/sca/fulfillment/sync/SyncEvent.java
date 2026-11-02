package gov.sca.fulfillment.sync;

import gov.sca.fulfillment.fulfillment.FulfillmentOrder;
import gov.sca.fulfillment.maintenance.MaintenanceRequest;
import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "sync_events")
public class SyncEvent {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "maintenance_request_id")
    private MaintenanceRequest maintenanceRequest;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "fulfillment_order_id")
    private FulfillmentOrder fulfillmentOrder;
    private String operation;
    @Enumerated(EnumType.STRING)
    private SyncDirection direction;
    private boolean success;
    private Integer httpStatus;
    private String errorCode;
    @Column(length = 1000)
    private String sanitizedMessage;
    private String correlationId;
    private Instant attemptedAt;
    private Instant completedAt;

    public Long getId() { return id; }
    public MaintenanceRequest getMaintenanceRequest() { return maintenanceRequest; }
    public FulfillmentOrder getFulfillmentOrder() { return fulfillmentOrder; }
    public String getOperation() { return operation; }
    public boolean isSuccess() { return success; }
    public Integer getHttpStatus() { return httpStatus; }
    public String getErrorCode() { return errorCode; }
    public String getSanitizedMessage() { return sanitizedMessage; }
    public String getCorrelationId() { return correlationId; }
    public Instant getAttemptedAt() { return attemptedAt; }
    public Instant getCompletedAt() { return completedAt; }
    public void setMaintenanceRequest(MaintenanceRequest maintenanceRequest) { this.maintenanceRequest = maintenanceRequest; }
    public void setFulfillmentOrder(FulfillmentOrder fulfillmentOrder) { this.fulfillmentOrder = fulfillmentOrder; }
    public void setOperation(String operation) { this.operation = operation; }
    public void setDirection(SyncDirection direction) { this.direction = direction; }
    public void setSuccess(boolean success) { this.success = success; }
    public void setHttpStatus(Integer httpStatus) { this.httpStatus = httpStatus; }
    public void setErrorCode(String errorCode) { this.errorCode = errorCode; }
    public void setSanitizedMessage(String sanitizedMessage) { this.sanitizedMessage = sanitizedMessage; }
    public void setCorrelationId(String correlationId) { this.correlationId = correlationId; }
    public void setAttemptedAt(Instant attemptedAt) { this.attemptedAt = attemptedAt; }
    public void setCompletedAt(Instant completedAt) { this.completedAt = completedAt; }
}
