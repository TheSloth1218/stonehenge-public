package gov.sca.fulfillment.fulfillment;

import gov.sca.fulfillment.maintenance.MaintenanceRequest;
import gov.sca.fulfillment.users.AppUser;
import jakarta.persistence.*;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

@Entity
@Table(name = "fulfillment_orders")
public class FulfillmentOrder {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    @OneToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "maintenance_request_id")
    private MaintenanceRequest maintenanceRequest;
    private String orderNumber;
    @Enumerated(EnumType.STRING)
    private OrderStatus status;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "assigned_user_id")
    private AppUser assignedUser;
    @Column(length = 2000)
    private String fulfillmentNotes;
    @Column(length = 2000)
    private String packingNotes;
    private Instant createdAt;
    private Instant updatedAt;
    private Instant reviewedAt;
    private Instant shippedAt;
    private Instant completedAt;
    @Version
    private long version;
    @OneToMany(mappedBy = "fulfillmentOrder", cascade = CascadeType.ALL, orphanRemoval = true)
    @OrderBy("addedAt asc")
    private List<FulfillmentOrderItem> items = new ArrayList<>();

    public boolean itemEditable() {
        return status == OrderStatus.DRAFT || status == OrderStatus.PICKING
                || status == OrderStatus.REVIEW || status == OrderStatus.READY_TO_SHIP;
    }

    public boolean shippable() {
        return status == OrderStatus.REVIEW || status == OrderStatus.READY_TO_SHIP;
    }

    public boolean terminal() {
        return status == OrderStatus.COMPLETION_PENDING || status == OrderStatus.COMPLETION_SENT
                || status == OrderStatus.COMPLETION_FAILED || status == OrderStatus.SHIPPED
                || status == OrderStatus.CANCELLED;
    }

    public Long getId() { return id; }
    public MaintenanceRequest getMaintenanceRequest() { return maintenanceRequest; }
    public String getOrderNumber() { return orderNumber; }
    public OrderStatus getStatus() { return status; }
    public AppUser getAssignedUser() { return assignedUser; }
    public String getFulfillmentNotes() { return fulfillmentNotes; }
    public String getPackingNotes() { return packingNotes; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }
    public Instant getReviewedAt() { return reviewedAt; }
    public Instant getShippedAt() { return shippedAt; }
    public Instant getCompletedAt() { return completedAt; }
    public long getVersion() { return version; }
    public List<FulfillmentOrderItem> getItems() { return items; }
    public void setId(Long id) { this.id = id; }
    public void setMaintenanceRequest(MaintenanceRequest maintenanceRequest) { this.maintenanceRequest = maintenanceRequest; }
    public void setOrderNumber(String orderNumber) { this.orderNumber = orderNumber; }
    public void setStatus(OrderStatus status) { this.status = status; }
    public void setAssignedUser(AppUser assignedUser) { this.assignedUser = assignedUser; }
    public void setFulfillmentNotes(String fulfillmentNotes) { this.fulfillmentNotes = fulfillmentNotes; }
    public void setPackingNotes(String packingNotes) { this.packingNotes = packingNotes; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }
    public void setReviewedAt(Instant reviewedAt) { this.reviewedAt = reviewedAt; }
    public void setShippedAt(Instant shippedAt) { this.shippedAt = shippedAt; }
    public void setCompletedAt(Instant completedAt) { this.completedAt = completedAt; }
}
