package gov.sca.fulfillment.fulfillment;

import gov.sca.fulfillment.common.PageUtils;
import gov.sca.fulfillment.inventory.InventoryRepository;
import jakarta.validation.Valid;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.validation.BindingResult;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.servlet.mvc.support.RedirectAttributes;
import gov.sca.fulfillment.shipping.ShipmentProgressService;

import java.util.HashMap;
import java.util.Map;

@Controller
@RequestMapping("/orders")
public class FulfillmentController {
    private final FulfillmentOrderRepository orders;
    private final FulfillmentService service;
    private final ShipmentProgressService shipmentProgress;
    private final InventoryRepository inventory;

    public FulfillmentController(FulfillmentOrderRepository orders, FulfillmentService service,
                                 ShipmentProgressService shipmentProgress, InventoryRepository inventory) {
        this.orders = orders;
        this.service = service;
        this.shipmentProgress = shipmentProgress;
        this.inventory = inventory;
    }

    @GetMapping
    public String list(@RequestParam(defaultValue = "0") int page, Model model) {
        model.addAttribute("orders", orders.findAllByOrderByUpdatedAtDesc(PageUtils.page(page, 15, "updatedAt")));
        return "fulfillment/list";
    }

    @GetMapping("/{id}/review")
    public String review(@PathVariable Long id, Model model) {
        FulfillmentOrder order = service.getOrder(id);
        addOrderModel(model, order);
        model.addAttribute("shipOrderForm", new ShipOrderForm("SCA Motor Pool", "Truck Transport", "Stonehenge Engineering Maintenance Bay", "", false,
                order.getMaintenanceRequest().getExternalRequestId()));
        return "fulfillment/review";
    }

    @PostMapping("/{id}/items")
    public String addItem(@PathVariable Long id, @Valid AddItemForm form, BindingResult binding, RedirectAttributes redirect) {
        if (binding.hasErrors()) {
            redirect.addFlashAttribute("error", "Select a part and a positive quantity.");
        } else {
            service.addItem(id, form.partId(), form.quantity());
        }
        return "redirect:/orders/" + id + "/review";
    }

    @PostMapping("/{id}/items/{itemId}")
    public String updateItem(@PathVariable Long id, @PathVariable Long itemId, @RequestParam int quantity) {
        service.updateQuantity(id, itemId, quantity);
        return "redirect:/orders/" + id + "/review";
    }

    @PostMapping("/{id}/items/{itemId}/remove")
    public String removeItem(@PathVariable Long id, @PathVariable Long itemId) {
        service.removeItem(id, itemId);
        return "redirect:/orders/" + id + "/review";
    }

    @PostMapping("/{id}/review")
    public String markReview(@PathVariable Long id) {
        service.markReview(id);
        return "redirect:/orders/" + id + "/review";
    }

    @PostMapping("/{id}/edit")
    public String returnToPicking(@PathVariable Long id) {
        service.returnToPicking(id);
        return "redirect:/orders/" + id + "/review";
    }

    @PostMapping("/{id}/ship")
    public String ship(@PathVariable Long id, @Valid ShipOrderForm form, BindingResult binding, Model model) {
        if (binding.hasErrors()) {
            addOrderModel(model, service.getOrder(id));
            model.addAttribute("shipOrderForm", form);
            return "fulfillment/review";
        }
        service.shipAndSync(id, form);
        return "redirect:/orders/" + id + "/shipping-status";
    }

    @GetMapping("/{id}/shipping-status")
    public String shippingStatus(@PathVariable Long id, Model model) {
        model.addAttribute("progress", shipmentProgress.get(id));
        return "fulfillment/shipping-status";
    }

    private void addOrderModel(Model model, FulfillmentOrder order) {
        Map<Long, Integer> availableStock = new HashMap<>();
        order.getItems().forEach(item -> availableStock.put(item.getPart().getId(), inventory
                .findByPartId(item.getPart().getId())
                .map(stock -> Math.max(stock.availableQuantity(), 0))
                .orElse(0)));
        model.addAttribute("order", order);
        model.addAttribute("availableStock", availableStock);
        model.addAttribute("hasInsufficientStock", order.getItems().stream()
                .anyMatch(item -> item.getQuantity() > availableStock.get(item.getPart().getId())));
    }
}
