package gov.sca.fulfillment.catalog;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "parts")
public class Part {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    private String sku;
    private String name;
    @Column(length = 2000)
    private String description;
    private String category;
    private String manufacturer;
    private String compatibleAssetClass;
    private String imagePath;
    private boolean active = true;
    private Instant createdAt;
    private Instant updatedAt;
    @Version
    private long version;

    public Long getId() { return id; }
    public String getSku() { return sku; }
    public String getName() { return name; }
    public String getDescription() { return description; }
    public String getCategory() { return category; }
    public String getManufacturer() { return manufacturer; }
    public String getCompatibleAssetClass() { return compatibleAssetClass; }
    public String getImagePath() { return imagePath; }
    public boolean isActive() { return active; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }
    public long getVersion() { return version; }
    public void setId(Long id) { this.id = id; }
    public void setSku(String sku) { this.sku = sku; }
    public void setName(String name) { this.name = name; }
    public void setDescription(String description) { this.description = description; }
    public void setCategory(String category) { this.category = category; }
    public void setManufacturer(String manufacturer) { this.manufacturer = manufacturer; }
    public void setCompatibleAssetClass(String compatibleAssetClass) { this.compatibleAssetClass = compatibleAssetClass; }
    public void setImagePath(String imagePath) { this.imagePath = imagePath; }
    public void setActive(boolean active) { this.active = active; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }
    public void setVersion(long version) { this.version = version; }
}
