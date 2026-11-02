package gov.sca.fulfillment.shipping;

import gov.sca.fulfillment.fulfillment.FulfillmentOrder;
import gov.sca.fulfillment.sync.SyncEvent;

public record ShipmentProgress(FulfillmentOrder order, Shipment shipment, SyncEvent syncEvent) {
    public boolean apiConfirmed() {
        return syncEvent != null && syncEvent.isSuccess();
    }

    public boolean apiFailed() {
        return syncEvent != null && !syncEvent.isSuccess();
    }
}
