package gov.sca.fulfillment.common;

import gov.sca.fulfillment.shipping.jobs.ShipmentJobProperties;
import gov.sca.fulfillment.sync.StonehengeApiProperties;
import gov.sca.fulfillment.sync.SyncEventRepository;
import org.springframework.core.env.Environment;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;

import javax.sql.DataSource;
import java.net.URI;

@Controller
@RequestMapping("/admin/health")
public class AdminHealthController {
    private final DataSource dataSource;
    private final StonehengeApiProperties api;
    private final ShipmentJobProperties jobs;
    private final SyncEventRepository syncEvents;
    private final Environment environment;

    public AdminHealthController(DataSource dataSource, StonehengeApiProperties api, ShipmentJobProperties jobs,
                                 SyncEventRepository syncEvents, Environment environment) {
        this.dataSource = dataSource;
        this.api = api;
        this.jobs = jobs;
        this.syncEvents = syncEvents;
        this.environment = environment;
    }

    @GetMapping
    @PreAuthorize("hasRole('ADMIN')")
    public String health(Model model) {
        model.addAttribute("databaseUp", databaseUp());
        model.addAttribute("apiConfigured", api.apiKey() != null && !api.apiKey().isBlank());
        model.addAttribute("apiHost", apiHost());
        model.addAttribute("lastSuccess", syncEvents.findFirstBySuccessTrueOrderByAttemptedAtDesc().orElse(null));
        model.addAttribute("lastFailure", syncEvents.findFirstBySuccessFalseOrderByAttemptedAtDesc().orElse(null));
        model.addAttribute("workerEnabled", jobs.worker().enabled());
        model.addAttribute("profiles", String.join(", ", environment.getActiveProfiles()));
        return "admin/health";
    }

    private boolean databaseUp() {
        try (var connection = dataSource.getConnection()) {
            return connection.isValid(2);
        } catch (Exception ex) {
            return false;
        }
    }

    private String apiHost() {
        try { return URI.create(api.baseUrl()).getHost(); }
        catch (IllegalArgumentException ex) { return "invalid configuration"; }
    }
}
