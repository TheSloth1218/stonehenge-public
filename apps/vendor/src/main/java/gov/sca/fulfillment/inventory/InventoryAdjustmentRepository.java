package gov.sca.fulfillment.inventory;

import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;
import org.springframework.data.jpa.domain.Specification;

public interface InventoryAdjustmentRepository extends JpaRepository<InventoryAdjustment, Long>, JpaSpecificationExecutor<InventoryAdjustment> {
    @EntityGraph(attributePaths = {"inventory", "inventory.part", "createdByUser"})
    Page<InventoryAdjustment> findAllByOrderByCreatedAtDesc(Pageable pageable);

    @Override
    @EntityGraph(attributePaths = {"inventory", "inventory.part", "createdByUser"})
    Page<InventoryAdjustment> findAll(Specification<InventoryAdjustment> specification, Pageable pageable);
}
