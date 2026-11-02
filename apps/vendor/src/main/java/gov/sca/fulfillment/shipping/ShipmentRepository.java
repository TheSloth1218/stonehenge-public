package gov.sca.fulfillment.shipping;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.Optional;

public interface ShipmentRepository extends JpaRepository<Shipment, Long> {
    Optional<Shipment> findByFulfillmentOrderId(Long orderId);

    @EntityGraph(attributePaths = {"fulfillmentOrder", "fulfillmentOrder.maintenanceRequest"})
    Page<Shipment> findAllByOrderByShippedAtDesc(Pageable pageable);
}
