package gov.sca.fulfillment.fulfillment;

import org.springframework.data.jpa.repository.JpaRepository;

import java.util.Optional;

public interface FulfillmentOrderItemRepository extends JpaRepository<FulfillmentOrderItem, Long> {
    Optional<FulfillmentOrderItem> findByFulfillmentOrderIdAndPartId(Long orderId, Long partId);
}
