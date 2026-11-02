package gov.sca.fulfillment.inventory;

import gov.sca.fulfillment.common.PageUtils;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import gov.sca.fulfillment.catalog.PartRepository;
import gov.sca.fulfillment.users.AppUserRepository;
import org.springframework.data.jpa.domain.Specification;

import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneOffset;

@Controller
@RequestMapping("/admin/inventory-adjustments")
public class InventoryAdminController {
    private final InventoryAdjustmentRepository adjustments;
    private final PartRepository parts;
    private final AppUserRepository users;

    public InventoryAdminController(InventoryAdjustmentRepository adjustments, PartRepository parts,
                                    AppUserRepository users) {
        this.adjustments = adjustments;
        this.parts = parts;
        this.users = users;
    }

    @GetMapping
    @PreAuthorize("hasRole('ADMIN')")
    public String list(@RequestParam(defaultValue = "0") int page,
                       @RequestParam(required = false) Long partId,
                       @RequestParam(required = false) Long employeeId,
                       @RequestParam(required = false) AdjustmentType type,
                       @RequestParam(required = false) LocalDate from,
                       @RequestParam(required = false) LocalDate to,
                       Model model) {
        Specification<InventoryAdjustment> filter = Specification.where(null);
        if (partId != null) filter = filter.and((root, query, cb) -> cb.equal(root.get("inventory").get("part").get("id"), partId));
        if (employeeId != null) filter = filter.and((root, query, cb) -> cb.equal(root.get("createdByUser").get("id"), employeeId));
        if (type != null) filter = filter.and((root, query, cb) -> cb.equal(root.get("adjustmentType"), type));
        if (from != null) {
            Instant start = from.atStartOfDay().toInstant(ZoneOffset.UTC);
            filter = filter.and((root, query, cb) -> cb.greaterThanOrEqualTo(root.get("createdAt"), start));
        }
        if (to != null) {
            Instant end = to.plusDays(1).atStartOfDay().toInstant(ZoneOffset.UTC);
            filter = filter.and((root, query, cb) -> cb.lessThan(root.get("createdAt"), end));
        }
        model.addAttribute("adjustments", adjustments.findAll(filter, PageUtils.page(page, 50, "createdAt")));
        model.addAttribute("parts", parts.findAll());
        model.addAttribute("users", users.findAll());
        model.addAttribute("types", AdjustmentType.values());
        model.addAttribute("partId", partId);
        model.addAttribute("employeeId", employeeId);
        model.addAttribute("type", type);
        model.addAttribute("from", from);
        model.addAttribute("to", to);
        return "admin/inventory-adjustments";
    }
}
