package gov.sca.fulfillment.shipping;

import gov.sca.fulfillment.common.PageUtils;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;

@Controller
@RequestMapping("/shipments")
public class ShippingController {
    private final ShipmentRepository shipments;

    public ShippingController(ShipmentRepository shipments) {
        this.shipments = shipments;
    }

    @GetMapping
    public String list(@RequestParam(defaultValue = "0") int page, Model model) {
        model.addAttribute("shipments", shipments.findAllByOrderByShippedAtDesc(PageUtils.page(page, 15, "shippedAt")));
        return "shipping/list";
    }
}
