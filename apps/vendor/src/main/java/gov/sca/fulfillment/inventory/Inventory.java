package gov.sca.fulfillment.inventory;

import gov.sca.fulfillment.catalog.Part;
import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "inventory")
public class Inventory {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    @OneToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "part_id")
    private Part part;
    private String warehouseLocation;
    private String binLocation;
    private int quantityOnHand;
    private int quantityReserved;
    private int reorderLevel;
    private Instant updatedAt;
    @Version
    private long version;

    public int availableQuantity() {
        return quantityOnHand - quantityReserved;
    }

    public Long getId() { return id; }
    public Part getPart() { return part; }
    public String getWarehouseLocation() { return warehouseLocation; }
    public String getBinLocation() { return binLocation; }
    public int getQuantityOnHand() { return quantityOnHand; }
    public int getQuantityReserved() { return quantityReserved; }
    public int getReorderLevel() { return reorderLevel; }
    public Instant getUpdatedAt() { return updatedAt; }
    public long getVersion() { return version; }
    public void setId(Long id) { this.id = id; }
    public void setPart(Part part) { this.part = part; }
    public void setWarehouseLocation(String warehouseLocation) { this.warehouseLocation = warehouseLocation; }
    public void setBinLocation(String binLocation) { this.binLocation = binLocation; }
    public void setQuantityOnHand(int quantityOnHand) { this.quantityOnHand = quantityOnHand; }
    public void setQuantityReserved(int quantityReserved) { this.quantityReserved = quantityReserved; }
    public void setReorderLevel(int reorderLevel) { this.reorderLevel = reorderLevel; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }
    public void setVersion(long version) { this.version = version; }
}
