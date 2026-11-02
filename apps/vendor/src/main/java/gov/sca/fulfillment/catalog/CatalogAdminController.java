package gov.sca.fulfillment.catalog;

import gov.sca.fulfillment.common.NotFoundException;
import gov.sca.fulfillment.common.PageUtils;
import jakarta.validation.Valid;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.validation.BindingResult;
import org.springframework.web.bind.annotation.*;

@Controller
@RequestMapping("/admin/parts")
public class CatalogAdminController {
    private final PartRepository parts;
    private final CatalogAdminService service;

    public CatalogAdminController(PartRepository parts, CatalogAdminService service) {
        this.parts = parts;
        this.service = service;
    }

    @GetMapping
    public String list(@RequestParam(defaultValue = "0") int page, Model model) {
        model.addAttribute("parts", parts.search("", null, null, PageUtils.page(page, 40, "name")));
        return "admin/parts/list";
    }

    @GetMapping("/new")
    public String createForm(Model model) {
        model.addAttribute("partForm", new PartAdminForm("", "", "", "", "", "", "/css/part-placeholder.svg", true));
        return "admin/parts/form";
    }

    @PostMapping
    public String create(@Valid @ModelAttribute("partForm") PartAdminForm form, BindingResult binding) {
        if (binding.hasErrors()) return "admin/parts/form";
        return "redirect:/admin/parts/" + service.create(form).getId() + "/edit";
    }

    @GetMapping("/{id}/edit")
    public String editForm(@PathVariable Long id, Model model) {
        Part part = parts.findById(id).orElseThrow(() -> new NotFoundException("Part was not found"));
        model.addAttribute("part", part);
        model.addAttribute("partForm", PartAdminForm.from(part));
        model.addAttribute("historicalReferences", service.historicalReferences(id));
        return "admin/parts/form";
    }

    @PostMapping("/{id}")
    public String update(@PathVariable Long id, @Valid @ModelAttribute("partForm") PartAdminForm form,
                         BindingResult binding, Model model) {
        if (binding.hasErrors()) {
            model.addAttribute("part", parts.findById(id).orElseThrow());
            model.addAttribute("historicalReferences", service.historicalReferences(id));
            return "admin/parts/form";
        }
        service.update(id, form);
        return "redirect:/admin/parts/" + id + "/edit";
    }

    @PostMapping("/{id}/active")
    public String active(@PathVariable Long id, @RequestParam boolean active) {
        service.setActive(id, active);
        return "redirect:/admin/parts/" + id + "/edit";
    }
}
