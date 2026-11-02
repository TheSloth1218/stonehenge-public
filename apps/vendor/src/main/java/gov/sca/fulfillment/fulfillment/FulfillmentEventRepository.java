package gov.sca.fulfillment.fulfillment;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;

public interface FulfillmentEventRepository extends JpaRepository<FulfillmentEvent, Long> {
    Page<FulfillmentEvent> findAllByOrderByCreatedAtDesc(Pageable pageable);
}
