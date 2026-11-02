package gov.sca.fulfillment.fulfillment;

import gov.sca.fulfillment.catalog.Part;
import gov.sca.fulfillment.users.AppUser;
import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "fulfillment_order_items")
public class FulfillmentOrderItem {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "fulfillment_order_id")
    private FulfillmentOrder fulfillmentOrder;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "part_id")
    private Part part;
    private int quantity;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "added_by_user_id")
    private AppUser addedByUser;
    private Instant addedAt;
    private Instant updatedAt;
    @Version
    private long version;

    public Long getId() { return id; }
    public FulfillmentOrder getFulfillmentOrder() { return fulfillmentOrder; }
    public Part getPart() { return part; }
    public int getQuantity() { return quantity; }
    public AppUser getAddedByUser() { return addedByUser; }
    public Instant getAddedAt() { return addedAt; }
    public Instant getUpdatedAt() { return updatedAt; }
    public void setId(Long id) { this.id = id; }
    public void setFulfillmentOrder(FulfillmentOrder fulfillmentOrder) { this.fulfillmentOrder = fulfillmentOrder; }
    public void setPart(Part part) { this.part = part; }
    public void setQuantity(int quantity) { this.quantity = quantity; }
    public void setAddedByUser(AppUser addedByUser) { this.addedByUser = addedByUser; }
    public void setAddedAt(Instant addedAt) { this.addedAt = addedAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }
}
