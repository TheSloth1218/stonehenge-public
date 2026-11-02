package gov.sca.fulfillment.maintenance;

import gov.sca.fulfillment.catalog.PartRepository;
import gov.sca.fulfillment.common.NotFoundException;
import gov.sca.fulfillment.common.PageUtils;
import gov.sca.fulfillment.fulfillment.FulfillmentOrder;
import gov.sca.fulfillment.fulfillment.FulfillmentOrderRepository;
import gov.sca.fulfillment.fulfillment.FulfillmentService;
import gov.sca.fulfillment.inventory.InventoryRepository;
import gov.sca.fulfillment.sync.VendorApiSyncService;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.*;

@Controller
@RequestMapping("/requests")
public class MaintenanceController {
    private final MaintenanceRequestRepository requests;
    private final FulfillmentOrderRepository orders;
    private final PartRepository parts;
    private final InventoryRepository inventory;
    private final FulfillmentService fulfillmentService;
    private final VendorApiSyncService vendorApiSync;

    public MaintenanceController(MaintenanceRequestRepository requests,
                                 FulfillmentOrderRepository orders,
                                 PartRepository parts,
                                 InventoryRepository inventory,
                                 FulfillmentService fulfillmentService,
                                 VendorApiSyncService vendorApiSync) {
        this.requests = requests;
        this.orders = orders;
        this.parts = parts;
        this.inventory = inventory;
        this.fulfillmentService = fulfillmentService;
        this.vendorApiSync = vendorApiSync;
    }

    @GetMapping
    public String list(@RequestParam(required = false) String q,
                       @RequestParam(required = false) LocalStatus status,
                       @RequestParam(required = false) Priority priority,
                       @RequestParam(defaultValue = "0") int page,
                       Model model) {
        model.addAttribute("requests", requests.search(PageUtils.searchTerm(q), status, priority, PageUtils.page(page, 15, "importedAt")));
        model.addAttribute("q", q);
        model.addAttribute("status", status);
        model.addAttribute("priority", priority);
        model.addAttribute("statuses", LocalStatus.values());
        model.addAttribute("priorities", Priority.values());
        model.addAttribute("apiState", vendorApiSync.state());
        return "maintenance/list";
    }

    @GetMapping("/{id}")
    public String detail(@PathVariable Long id,
                         @RequestParam(required = false) String partSearch,
                         @RequestParam(required = false) String category,
                         Model model) {
        MaintenanceRequest request = requests.findWithAssignedUserById(id).orElseThrow(() -> new NotFoundException("Maintenance request was not found"));
        FulfillmentOrder order = orders.findByMaintenanceRequestId(id).flatMap(o -> orders.findWithItemsById(o.getId())).orElse(null);
        model.addAttribute("request", request);
        model.addAttribute("order", order);
        model.addAttribute("catalog", parts.search(PageUtils.searchTerm(partSearch), PageUtils.blankToNull(category), true, PageRequest.of(0, 10)));
        model.addAttribute("categories", parts.categories());
        model.addAttribute("partSearch", partSearch);
        model.addAttribute("category", category);
        model.addAttribute("inventoryRepository", inventory);
        model.addAttribute("apiState", vendorApiSync.state());
        return "maintenance/detail";
    }

    @PostMapping("/{id}/begin")
    public String begin(@PathVariable Long id) {
        fulfillmentService.acknowledgeAndBegin(id);
        return "redirect:/requests/" + id;
    }

    @PostMapping("/sync")
    public String sync() {
        vendorApiSync.refreshNow();
        return "redirect:/requests";
    }

    @PostMapping("/{id}/items")
    public String addItem(@PathVariable Long id, @RequestParam Long partId, @RequestParam(defaultValue = "1") int quantity) {
        fulfillmentService.addItemForRequest(id, partId, quantity);
        return "redirect:/requests/" + id;
    }

    @PostMapping("/{id}/orders/{orderId}/items/{itemId}/remove")
    public String removeItem(@PathVariable Long id, @PathVariable Long orderId, @PathVariable Long itemId) {
        requireRequestOrder(id, orderId);
        fulfillmentService.removeItem(orderId, itemId);
        return "redirect:/requests/" + id;
    }

    @PostMapping("/{id}/orders/{orderId}/clear")
    public String clearOrder(@PathVariable Long id, @PathVariable Long orderId) {
        requireRequestOrder(id, orderId);
        fulfillmentService.clearItems(orderId);
        return "redirect:/requests/" + id;
    }

    private void requireRequestOrder(Long requestId, Long orderId) {
        FulfillmentOrder order = orders.findByMaintenanceRequestId(requestId)
                .orElseThrow(() -> new NotFoundException("Fulfillment order was not found"));
        if (!order.getId().equals(orderId)) {
            throw new NotFoundException("Fulfillment order was not found");
        }
    }
}
