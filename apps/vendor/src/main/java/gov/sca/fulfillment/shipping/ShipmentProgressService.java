package gov.sca.fulfillment.shipping;

import gov.sca.fulfillment.common.NotFoundException;
import gov.sca.fulfillment.fulfillment.FulfillmentOrder;
import gov.sca.fulfillment.fulfillment.FulfillmentService;
import gov.sca.fulfillment.sync.SyncEvent;
import gov.sca.fulfillment.sync.SyncEventRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class ShipmentProgressService {
    private final FulfillmentService fulfillment;
    private final ShipmentRepository shipments;
    private final SyncEventRepository syncEvents;

    public ShipmentProgressService(FulfillmentService fulfillment, ShipmentRepository shipments,
                                   SyncEventRepository syncEvents) {
        this.fulfillment = fulfillment;
        this.shipments = shipments;
        this.syncEvents = syncEvents;
    }

    @Transactional(readOnly = true)
    public ShipmentProgress get(Long orderId) {
        FulfillmentOrder order = fulfillment.getOrder(orderId);
        Shipment shipment = shipments.findByFulfillmentOrderId(orderId)
                .orElseThrow(() -> new NotFoundException("Shipment was not found for this order"));
        SyncEvent sync = syncEvents.findFirstByFulfillmentOrderIdOrderByAttemptedAtDesc(orderId).orElse(null);
        return new ShipmentProgress(order, shipment, sync);
    }
}
