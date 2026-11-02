package gov.sca.fulfillment.fulfillment;

import jakarta.persistence.LockModeType;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.Optional;

public interface FulfillmentOrderRepository extends JpaRepository<FulfillmentOrder, Long> {
    Optional<FulfillmentOrder> findByMaintenanceRequestId(Long requestId);

    Optional<FulfillmentOrder> findByOrderNumber(String orderNumber);

    @EntityGraph(attributePaths = {"maintenanceRequest", "assignedUser", "items", "items.part"})
    @Query("select o from FulfillmentOrder o where o.id = :id")
    Optional<FulfillmentOrder> findWithItemsById(@Param("id") Long id);

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select o from FulfillmentOrder o where o.id = :id")
    Optional<FulfillmentOrder> lockById(@Param("id") Long id);

    @EntityGraph(attributePaths = {"maintenanceRequest", "assignedUser"})
    Page<FulfillmentOrder> findAllByOrderByUpdatedAtDesc(Pageable pageable);
}
