package gov.sca.fulfillment.sync;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.EntityGraph;

import java.util.Optional;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface SyncEventRepository extends JpaRepository<SyncEvent, Long> {
    @EntityGraph(attributePaths = {"maintenanceRequest", "fulfillmentOrder"})
    Page<SyncEvent> findAllByOrderByAttemptedAtDesc(Pageable pageable);

    @EntityGraph(attributePaths = {"maintenanceRequest", "fulfillmentOrder"})
    @Query("select s from SyncEvent s where (:success is null or s.success = :success) order by s.attemptedAt desc")
    Page<SyncEvent> search(@Param("success") Boolean success, Pageable pageable);

    @EntityGraph(attributePaths = {"maintenanceRequest", "fulfillmentOrder"})
    Optional<SyncEvent> findDetailedById(Long id);

    Optional<SyncEvent> findFirstBySuccessTrueOrderByAttemptedAtDesc();
    Optional<SyncEvent> findFirstBySuccessFalseOrderByAttemptedAtDesc();

    @EntityGraph(attributePaths = {"maintenanceRequest", "fulfillmentOrder"})
    Optional<SyncEvent> findFirstByFulfillmentOrderIdOrderByAttemptedAtDesc(Long fulfillmentOrderId);
}
