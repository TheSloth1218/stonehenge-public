package gov.sca.fulfillment.common;

import gov.sca.fulfillment.catalog.PartRepository;
import gov.sca.fulfillment.inventory.InventoryAdjustmentRepository;
import gov.sca.fulfillment.shipping.jobs.ShipmentJobRepository;
import gov.sca.fulfillment.sync.SyncEventRepository;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;

@Controller
@RequestMapping("/admin")
public class AdminController {
    private final PartRepository parts;
    private final InventoryAdjustmentRepository adjustments;
    private final SyncEventRepository syncEvents;
    private final ShipmentJobRepository jobs;

    public AdminController(PartRepository parts, InventoryAdjustmentRepository adjustments,
                           SyncEventRepository syncEvents, ShipmentJobRepository jobs) {
        this.parts = parts;
        this.adjustments = adjustments;
        this.syncEvents = syncEvents;
        this.jobs = jobs;
    }

    @GetMapping
    public String admin(Model model) {
        model.addAttribute("partCount", parts.count());
        model.addAttribute("adjustmentCount", adjustments.count());
        model.addAttribute("syncCount", syncEvents.count());
        model.addAttribute("jobCount", jobs.count());
        return "admin/index";
    }
}
