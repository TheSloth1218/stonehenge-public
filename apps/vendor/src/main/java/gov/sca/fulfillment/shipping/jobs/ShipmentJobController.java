package gov.sca.fulfillment.shipping.jobs;

import gov.sca.fulfillment.common.PageUtils;
import jakarta.validation.Valid;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.validation.BindingResult;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@Controller
@RequestMapping("/admin/shipment-jobs")
public class ShipmentJobController {
    private final ShipmentJobService service;

    public ShipmentJobController(ShipmentJobService service) {
        this.service = service;
    }

    @GetMapping
    public String list(@RequestParam(defaultValue = "0") int page, Model model) {
        model.addAttribute("jobs", service.list(PageUtils.page(page, 30, "createdAt")));
        return "admin/shipment-jobs/list";
    }

    @GetMapping("/{id}")
    public String detail(@PathVariable Long id, Model model) {
        model.addAttribute("job", service.detail(id));
        return "admin/shipment-jobs/detail";
    }

    @GetMapping(value = "/{id}/export", produces = MediaType.TEXT_PLAIN_VALUE)
    @ResponseBody
    public String export(@PathVariable Long id) {
        return service.export(id);
    }

    @PostMapping(value = "/replay", consumes = MediaType.APPLICATION_JSON_VALUE, produces = MediaType.APPLICATION_JSON_VALUE)
    @ResponseBody
    public ResponseEntity<Map<String, Object>> replayJson(@Valid @RequestBody ReplayShipmentJobForm form) {
        ShipmentJob job = service.replay(form.payload());
        return ResponseEntity.accepted().body(Map.of("jobId", job.getId(), "status", job.getStatus()));
    }

    @PostMapping("/replay")
    public String replayForm(@Valid ReplayShipmentJobForm form, BindingResult binding) {
        if (binding.hasErrors()) {
            return "redirect:/admin/shipment-jobs";
        }
        ShipmentJob job = service.replay(form.payload());
        return "redirect:/admin/shipment-jobs/" + job.getId();
    }
}
