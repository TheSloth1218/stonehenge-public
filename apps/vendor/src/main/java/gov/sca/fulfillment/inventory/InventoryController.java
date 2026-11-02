package gov.sca.fulfillment.inventory;

import gov.sca.fulfillment.common.AppException;
import gov.sca.fulfillment.common.NotFoundException;
import gov.sca.fulfillment.common.PageUtils;
import gov.sca.fulfillment.fulfillment.EventType;
import gov.sca.fulfillment.fulfillment.FulfillmentEvent;
import gov.sca.fulfillment.fulfillment.FulfillmentEventRepository;
import gov.sca.fulfillment.users.CurrentUserService;
import org.springframework.stereotype.Controller;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.*;

import java.time.Instant;

@Controller
@RequestMapping("/inventory")
public class InventoryController {
    private final InventoryRepository inventory;
    private final InventoryAdjustmentRepository adjustments;
    private final CurrentUserService currentUserService;
    private final FulfillmentEventRepository events;

    public InventoryController(InventoryRepository inventory,
                               InventoryAdjustmentRepository adjustments,
                               CurrentUserService currentUserService,
                               FulfillmentEventRepository events) {
        this.inventory = inventory;
        this.adjustments = adjustments;
        this.currentUserService = currentUserService;
        this.events = events;
    }

    @GetMapping
    public String list(@RequestParam(required = false) String q,
                       @RequestParam(defaultValue = "0") int page,
                       Model model) {
        model.addAttribute("inventory", inventory.search(PageUtils.searchTerm(q), PageUtils.page(page, 15, "updatedAt")));
        model.addAttribute("q", q);
        return "inventory/list";
    }

    @PostMapping("/{id}/adjust")
    @Transactional
    public String adjust(@PathVariable Long id, @RequestParam int quantityDelta, @RequestParam String reason) {
        if (reason == null || reason.isBlank() || reason.length() > 300) {
            throw new AppException("Adjustment reason is required and must be concise.");
        }
        Inventory item = inventory.findById(id).orElseThrow(() -> new NotFoundException("Inventory record was not found"));
        int previous = item.getQuantityOnHand();
        int resulting = previous + quantityDelta;
        if (resulting < 0) {
            throw new AppException("Inventory cannot be adjusted below zero.");
        }
        item.setQuantityOnHand(resulting);
        item.setUpdatedAt(Instant.now());
        InventoryAdjustment adjustment = new InventoryAdjustment();
        adjustment.setInventory(item);
        adjustment.setAdjustmentType(AdjustmentType.MANUAL_CORRECTION);
        adjustment.setQuantityDelta(quantityDelta);
        adjustment.setPreviousQuantity(previous);
        adjustment.setResultingQuantity(resulting);
        adjustment.setReason(reason.trim());
        adjustment.setCreatedByUser(currentUserService.currentUser());
        adjustment.setCreatedAt(Instant.now());
        adjustments.save(adjustment);
        FulfillmentEvent event = new FulfillmentEvent();
        event.setEventType(EventType.STOCK_MANUALLY_ADJUSTED);
        event.setActorUser(currentUserService.currentUser());
        event.setSummary("Adjusted " + item.getPart().getSku() + " by " + quantityDelta);
        event.setDetailsJson("{}");
        event.setCreatedAt(Instant.now());
        events.save(event);
        return "redirect:/inventory";
    }
}
