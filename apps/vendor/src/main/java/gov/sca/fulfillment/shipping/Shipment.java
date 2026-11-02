package gov.sca.fulfillment.shipping;

import gov.sca.fulfillment.fulfillment.FulfillmentOrder;
import gov.sca.fulfillment.users.AppUser;
import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "shipments")
public class Shipment {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    @OneToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "fulfillment_order_id")
    private FulfillmentOrder fulfillmentOrder;
    private String shipmentNumber;
    private String carrier;
    private String shippingMethod;
    private String trackingNumber;
    private String destinationLabel;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "packed_by_user_id")
    private AppUser packedByUser;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "shipped_by_user_id")
    private AppUser shippedByUser;
    private Instant shippedAt;
    @Column(length = 2000)
    private String notes;
    private Instant createdAt;

    public Long getId() { return id; }
    public FulfillmentOrder getFulfillmentOrder() { return fulfillmentOrder; }
    public String getShipmentNumber() { return shipmentNumber; }
    public String getCarrier() { return carrier; }
    public String getShippingMethod() { return shippingMethod; }
    public String getTrackingNumber() { return trackingNumber; }
    public String getDestinationLabel() { return destinationLabel; }
    public AppUser getPackedByUser() { return packedByUser; }
    public AppUser getShippedByUser() { return shippedByUser; }
    public Instant getShippedAt() { return shippedAt; }
    public String getNotes() { return notes; }
    public Instant getCreatedAt() { return createdAt; }
    public void setFulfillmentOrder(FulfillmentOrder fulfillmentOrder) { this.fulfillmentOrder = fulfillmentOrder; }
    public void setShipmentNumber(String shipmentNumber) { this.shipmentNumber = shipmentNumber; }
    public void setCarrier(String carrier) { this.carrier = carrier; }
    public void setShippingMethod(String shippingMethod) { this.shippingMethod = shippingMethod; }
    public void setTrackingNumber(String trackingNumber) { this.trackingNumber = trackingNumber; }
    public void setDestinationLabel(String destinationLabel) { this.destinationLabel = destinationLabel; }
    public void setPackedByUser(AppUser packedByUser) { this.packedByUser = packedByUser; }
    public void setShippedByUser(AppUser shippedByUser) { this.shippedByUser = shippedByUser; }
    public void setShippedAt(Instant shippedAt) { this.shippedAt = shippedAt; }
    public void setNotes(String notes) { this.notes = notes; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
}
