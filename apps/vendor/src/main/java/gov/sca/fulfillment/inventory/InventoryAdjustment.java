package gov.sca.fulfillment.inventory;

import gov.sca.fulfillment.users.AppUser;
import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "inventory_adjustments")
public class InventoryAdjustment {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "inventory_id")
    private Inventory inventory;
    @Enumerated(EnumType.STRING)
    private AdjustmentType adjustmentType;
    private int quantityDelta;
    private int previousQuantity;
    private int resultingQuantity;
    private String reason;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "created_by_user_id")
    private AppUser createdByUser;
    private Instant createdAt;

    public Long getId() { return id; }
    public Inventory getInventory() { return inventory; }
    public AdjustmentType getAdjustmentType() { return adjustmentType; }
    public int getQuantityDelta() { return quantityDelta; }
    public int getPreviousQuantity() { return previousQuantity; }
    public int getResultingQuantity() { return resultingQuantity; }
    public String getReason() { return reason; }
    public AppUser getCreatedByUser() { return createdByUser; }
    public Instant getCreatedAt() { return createdAt; }
    public void setInventory(Inventory inventory) { this.inventory = inventory; }
    public void setAdjustmentType(AdjustmentType adjustmentType) { this.adjustmentType = adjustmentType; }
    public void setQuantityDelta(int quantityDelta) { this.quantityDelta = quantityDelta; }
    public void setPreviousQuantity(int previousQuantity) { this.previousQuantity = previousQuantity; }
    public void setResultingQuantity(int resultingQuantity) { this.resultingQuantity = resultingQuantity; }
    public void setReason(String reason) { this.reason = reason; }
    public void setCreatedByUser(AppUser createdByUser) { this.createdByUser = createdByUser; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
}
