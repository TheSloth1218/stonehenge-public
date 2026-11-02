package gov.sca.fulfillment.common;

import gov.sca.fulfillment.fulfillment.FulfillmentEventRepository;
import gov.sca.fulfillment.fulfillment.FulfillmentOrderRepository;
import gov.sca.fulfillment.inventory.InventoryRepository;
import gov.sca.fulfillment.maintenance.LocalStatus;
import gov.sca.fulfillment.maintenance.MaintenanceRequestRepository;
import gov.sca.fulfillment.shipping.ShipmentRepository;
import gov.sca.fulfillment.sync.SyncEventRepository;
import gov.sca.fulfillment.sync.VendorApiSyncService;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;

@Controller
public class DashboardController {
    private final MaintenanceRequestRepository requests;
    private final FulfillmentOrderRepository orders;
    private final ShipmentRepository shipments;
    private final InventoryRepository inventory;
    private final FulfillmentEventRepository events;
    private final SyncEventRepository syncEvents;
    private final VendorApiSyncService vendorApiSync;

    public DashboardController(MaintenanceRequestRepository requests,
                               FulfillmentOrderRepository orders,
                               ShipmentRepository shipments,
                               InventoryRepository inventory,
                               FulfillmentEventRepository events,
                               SyncEventRepository syncEvents,
                               VendorApiSyncService vendorApiSync) {
        this.requests = requests;
        this.orders = orders;
        this.shipments = shipments;
        this.inventory = inventory;
        this.events = events;
        this.syncEvents = syncEvents;
        this.vendorApiSync = vendorApiSync;
    }

    @GetMapping({"/", "/dashboard"})
    public String dashboard(Model model) {
        model.addAttribute("newRequests", requests.search("", LocalStatus.NEW, null, PageRequest.of(0, 5)));
        model.addAttribute("activeOrders", orders.findAllByOrderByUpdatedAtDesc(PageRequest.of(0, 5)));
        model.addAttribute("shipments", shipments.findAllByOrderByShippedAtDesc(PageRequest.of(0, 5)));
        model.addAttribute("inventory", inventory.search("", PageRequest.of(0, 8)));
        model.addAttribute("events", events.findAllByOrderByCreatedAtDesc(PageRequest.of(0, 8)));
        model.addAttribute("syncEvents", syncEvents.findAllByOrderByAttemptedAtDesc(PageRequest.of(0, 5)));
        model.addAttribute("gunStatuses", vendorApiSync.gunStatuses());
        model.addAttribute("apiState", vendorApiSync.state());
        return "dashboard";
    }
}
