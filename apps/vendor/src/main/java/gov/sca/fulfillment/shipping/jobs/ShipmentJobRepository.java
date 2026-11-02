package gov.sca.fulfillment.shipping.jobs;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

import java.util.Optional;

public interface ShipmentJobRepository extends JpaRepository<ShipmentJob, Long> {
    @Query(value = "select * from shipment_jobs where status in ('QUEUED', 'RETRYABLE_FAILED') order by created_at asc for update skip locked limit 1", nativeQuery = true)
    Optional<ShipmentJob> lockNextProcessable();

    @EntityGraph(attributePaths = {"shipment", "fulfillmentOrder", "maintenanceRequest", "createdByUser", "replayedByUser"})
    Page<ShipmentJob> findAllByOrderByCreatedAtDesc(Pageable pageable);

    @EntityGraph(attributePaths = {"shipment", "fulfillmentOrder", "maintenanceRequest", "createdByUser", "replayedByUser"})
    @Query("select j from ShipmentJob j where j.id = :id")
    Optional<ShipmentJob> findDetailedById(Long id);
}
