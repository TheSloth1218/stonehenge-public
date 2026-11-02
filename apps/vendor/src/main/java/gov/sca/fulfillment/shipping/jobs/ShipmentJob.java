package gov.sca.fulfillment.shipping.jobs;

import gov.sca.fulfillment.fulfillment.FulfillmentOrder;
import gov.sca.fulfillment.maintenance.MaintenanceRequest;
import gov.sca.fulfillment.shipping.Shipment;
import gov.sca.fulfillment.users.AppUser;
import jakarta.persistence.*;

import java.time.Instant;

@Entity
@Table(name = "shipment_jobs")
public class ShipmentJob {
    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    @ManyToOne(fetch = FetchType.LAZY) @JoinColumn(name = "shipment_id")
    private Shipment shipment;
    @ManyToOne(fetch = FetchType.LAZY) @JoinColumn(name = "fulfillment_order_id")
    private FulfillmentOrder fulfillmentOrder;
    @ManyToOne(fetch = FetchType.LAZY) @JoinColumn(name = "maintenance_request_id")
    private MaintenanceRequest maintenanceRequest;
    private String externalRequestId;
    private String payloadFormat;
    @Column(columnDefinition = "text")
    private String encodedPayload;
    @Enumerated(EnumType.STRING)
    private ShipmentJobStatus status;
    private int attemptCount;
    @ManyToOne(fetch = FetchType.LAZY) @JoinColumn(name = "created_by_user_id")
    private AppUser createdByUser;
    @ManyToOne(fetch = FetchType.LAZY) @JoinColumn(name = "replayed_by_user_id")
    private AppUser replayedByUser;
    private Instant createdAt;
    private Instant claimedAt;
    private Instant processedAt;
    private String lastError;
    @Version
    private long version;

    public Long getId() { return id; }
    public Shipment getShipment() { return shipment; }
    public FulfillmentOrder getFulfillmentOrder() { return fulfillmentOrder; }
    public MaintenanceRequest getMaintenanceRequest() { return maintenanceRequest; }
    public String getExternalRequestId() { return externalRequestId; }
    public String getPayloadFormat() { return payloadFormat; }
    public String getEncodedPayload() { return encodedPayload; }
    public ShipmentJobStatus getStatus() { return status; }
    public int getAttemptCount() { return attemptCount; }
    public AppUser getCreatedByUser() { return createdByUser; }
    public AppUser getReplayedByUser() { return replayedByUser; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getClaimedAt() { return claimedAt; }
    public Instant getProcessedAt() { return processedAt; }
    public String getLastError() { return lastError; }
    public void setShipment(Shipment shipment) { this.shipment = shipment; }
    public void setFulfillmentOrder(FulfillmentOrder fulfillmentOrder) { this.fulfillmentOrder = fulfillmentOrder; }
    public void setMaintenanceRequest(MaintenanceRequest maintenanceRequest) { this.maintenanceRequest = maintenanceRequest; }
    public void setExternalRequestId(String externalRequestId) { this.externalRequestId = externalRequestId; }
    public void setPayloadFormat(String payloadFormat) { this.payloadFormat = payloadFormat; }
    public void setEncodedPayload(String encodedPayload) { this.encodedPayload = encodedPayload; }
    public void setStatus(ShipmentJobStatus status) { this.status = status; }
    public void setAttemptCount(int attemptCount) { this.attemptCount = attemptCount; }
    public void setCreatedByUser(AppUser createdByUser) { this.createdByUser = createdByUser; }
    public void setReplayedByUser(AppUser replayedByUser) { this.replayedByUser = replayedByUser; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public void setClaimedAt(Instant claimedAt) { this.claimedAt = claimedAt; }
    public void setProcessedAt(Instant processedAt) { this.processedAt = processedAt; }
    public void setLastError(String lastError) { this.lastError = lastError; }
}
