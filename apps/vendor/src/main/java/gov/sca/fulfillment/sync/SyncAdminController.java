package gov.sca.fulfillment.sync;

import gov.sca.fulfillment.common.PageUtils;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.*;

@Controller
@RequestMapping("/admin/sync")
public class SyncAdminController {
    private final SyncEventRepository syncEvents;
    private final SyncAdminService service;
    private final StonehengeApiProperties api;

    public SyncAdminController(SyncEventRepository syncEvents, SyncAdminService service, StonehengeApiProperties api) {
        this.syncEvents = syncEvents;
        this.service = service;
        this.api = api;
    }

    @GetMapping
    public String list(@RequestParam(defaultValue = "0") int page,
                       @RequestParam(required = false) Boolean success, Model model) {
        model.addAttribute("syncEvents", syncEvents.search(success, PageUtils.page(page, 40, "attemptedAt")));
        model.addAttribute("success", success);
        return "admin/sync/list";
    }

    @GetMapping("/{id}")
    public String detail(@PathVariable Long id, Model model) {
        model.addAttribute("sync", service.detail(id));
        model.addAttribute("apiHost", redactedHost());
        model.addAttribute("serviceAccount", "stonevendor");
        model.addAttribute("apiKeyEnvironment", "STONEHENGE_VENDOR_API_KEY");
        return "admin/sync/detail";
    }

    @PostMapping("/{id}/retry")
    public String retry(@PathVariable Long id) {
        service.retry(id);
        return "redirect:/admin/sync";
    }

    private String redactedHost() {
        try {
            return java.net.URI.create(api.baseUrl()).getHost();
        } catch (IllegalArgumentException ex) {
            return "invalid configuration";
        }
    }
}
