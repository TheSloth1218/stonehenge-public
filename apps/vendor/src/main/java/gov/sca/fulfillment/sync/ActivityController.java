package gov.sca.fulfillment.sync;

import gov.sca.fulfillment.common.PageUtils;
import gov.sca.fulfillment.fulfillment.FulfillmentEventRepository;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;

@Controller
@RequestMapping("/activity")
public class ActivityController {
    private final FulfillmentEventRepository events;
    private final SyncEventRepository syncEvents;

    public ActivityController(FulfillmentEventRepository events, SyncEventRepository syncEvents) {
        this.events = events;
        this.syncEvents = syncEvents;
    }

    @GetMapping
    public String list(@RequestParam(defaultValue = "0") int page, Model model) {
        model.addAttribute("events", events.findAllByOrderByCreatedAtDesc(PageUtils.page(page, 20, "createdAt")));
        model.addAttribute("syncEvents", syncEvents.findAllByOrderByAttemptedAtDesc(PageUtils.page(page, 20, "attemptedAt")));
        return "sync/activity";
    }
}
