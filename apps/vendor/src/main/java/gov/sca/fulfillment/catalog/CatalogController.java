package gov.sca.fulfillment.catalog;

import gov.sca.fulfillment.common.NotFoundException;
import gov.sca.fulfillment.common.PageUtils;
import gov.sca.fulfillment.inventory.InventoryRepository;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.*;

@Controller
@RequestMapping("/catalog")
public class CatalogController {
    private final PartRepository parts;
    private final InventoryRepository inventory;

    public CatalogController(PartRepository parts, InventoryRepository inventory) {
        this.parts = parts;
        this.inventory = inventory;
    }

    @GetMapping
    public String list(@RequestParam(required = false) String q,
                       @RequestParam(required = false) String category,
                       @RequestParam(defaultValue = "0") int page,
                       Model model) {
        model.addAttribute("parts", parts.search(PageUtils.searchTerm(q), PageUtils.blankToNull(category), null, PageUtils.page(page, 15, "name")));
        model.addAttribute("categories", parts.categories());
        model.addAttribute("inventoryRepository", inventory);
        model.addAttribute("q", q);
        model.addAttribute("category", category);
        return "catalog/list";
    }

    @GetMapping("/{id}")
    public String detail(@PathVariable Long id, Model model) {
        Part part = parts.findById(id).orElseThrow(() -> new NotFoundException("Part was not found"));
        model.addAttribute("part", part);
        model.addAttribute("inventory", inventory.findByPartId(id).orElse(null));
        return "catalog/detail";
    }
}
