package gov.sca.fulfillment.catalog;

import gov.sca.fulfillment.common.AppException;
import gov.sca.fulfillment.common.NotFoundException;
import gov.sca.fulfillment.inventory.Inventory;
import gov.sca.fulfillment.inventory.InventoryRepository;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;

@Service
public class CatalogAdminService {
    private final PartRepository parts;
    private final InventoryRepository inventory;

    public CatalogAdminService(PartRepository parts, InventoryRepository inventory) {
        this.parts = parts;
        this.inventory = inventory;
    }

    @Transactional
    @PreAuthorize("hasRole('ADMIN')")
    public Part create(PartAdminForm form) {
        if (parts.findBySku(form.sku().trim()).isPresent()) {
            throw new AppException("A part with that SKU already exists.");
        }
        Part part = new Part();
        apply(part, form);
        Instant now = Instant.now();
        part.setCreatedAt(now);
        part.setUpdatedAt(now);
        parts.save(part);

        Inventory stock = new Inventory();
        stock.setPart(part);
        stock.setWarehouseLocation("SCA Logistics Cage A");
        stock.setBinLocation("UNASSIGNED");
        stock.setQuantityOnHand(0);
        stock.setQuantityReserved(0);
        stock.setReorderLevel(0);
        stock.setUpdatedAt(now);
        inventory.save(stock);
        return part;
    }

    @Transactional
    @PreAuthorize("hasRole('ADMIN')")
    public Part update(Long id, PartAdminForm form) {
        Part part = parts.findById(id).orElseThrow(() -> new NotFoundException("Part was not found"));
        parts.findBySku(form.sku().trim()).filter(existing -> !existing.getId().equals(id)).ifPresent(existing -> {
            throw new AppException("A part with that SKU already exists.");
        });
        apply(part, form);
        part.setUpdatedAt(Instant.now());
        return part;
    }

    @Transactional
    @PreAuthorize("hasRole('ADMIN')")
    public void setActive(Long id, boolean active) {
        Part part = parts.findById(id).orElseThrow(() -> new NotFoundException("Part was not found"));
        part.setActive(active);
        part.setUpdatedAt(Instant.now());
    }

    @Transactional(readOnly = true)
    @PreAuthorize("hasRole('ADMIN')")
    public long historicalReferences(Long id) {
        return parts.historicalReferenceCount(id);
    }

    private void apply(Part part, PartAdminForm form) {
        part.setSku(form.sku().trim());
        part.setName(form.name().trim());
        part.setDescription(clean(form.description()));
        part.setCategory(form.category().trim());
        part.setManufacturer(clean(form.manufacturer()));
        part.setCompatibleAssetClass(clean(form.compatibleAssetClass()));
        part.setImagePath(clean(form.imagePath()));
        part.setActive(form.active());
    }

    private String clean(String value) {
        return value == null || value.isBlank() ? null : value.trim();
    }
}
