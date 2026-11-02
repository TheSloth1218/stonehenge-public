package gov.sca.fulfillment.catalog;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record PartAdminForm(
        @NotBlank @Size(max = 80) String sku,
        @NotBlank @Size(max = 180) String name,
        @Size(max = 2000) String description,
        @NotBlank @Size(max = 100) String category,
        @Size(max = 120) String manufacturer,
        @Size(max = 120) String compatibleAssetClass,
        @Size(max = 240) String imagePath,
        boolean active
) {
    static PartAdminForm from(Part part) {
        return new PartAdminForm(part.getSku(), part.getName(), part.getDescription(), part.getCategory(),
                part.getManufacturer(), part.getCompatibleAssetClass(), part.getImagePath(), part.isActive());
    }
}
