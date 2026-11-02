package gov.sca.fulfillment.security;

import org.junit.jupiter.api.Test;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.config.annotation.web.configuration.EnableWebSecurity;
import org.springframework.web.servlet.config.annotation.EnableWebMvc;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.csrf;
import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.user;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

class TrainingRouteProfileTest {
    @Test
    void employeeCanReachOnlyTheDocumentedTrainingRoutes() throws Exception {
        MockMvc mvc = mvc(true);
        mvc.perform(get("/actuator/mappings").with(user("employee").roles("EMPLOYEE"))).andExpect(status().isOk());
        mvc.perform(get("/admin/shipment-jobs/7/export").with(user("employee").roles("EMPLOYEE"))).andExpect(status().isOk());
        mvc.perform(post("/admin/shipment-jobs/replay").with(user("employee").roles("EMPLOYEE")).with(csrf())).andExpect(status().isOk());
        mvc.perform(get("/admin/sync/9").with(user("employee").roles("EMPLOYEE"))).andExpect(status().isOk());
        mvc.perform(post("/admin/sync/9/retry").with(user("employee").roles("EMPLOYEE")).with(csrf())).andExpect(status().isOk());
        mvc.perform(get("/admin/sync").with(user("employee").roles("EMPLOYEE"))).andExpect(status().isForbidden());
    }

    @Test
    void safeProfileDeniesEmployeeAndAllowsAdminOnAllSensitiveRoutes() throws Exception {
        MockMvc mvc = mvc(false);
        for (String path : new String[]{"/actuator/mappings", "/admin/shipment-jobs/7/export", "/admin/sync/9"}) {
            mvc.perform(get(path).with(user("employee").roles("EMPLOYEE"))).andExpect(status().isForbidden());
            mvc.perform(get(path).with(user("admin").roles("EMPLOYEE", "ADMIN"))).andExpect(status().isOk());
        }
        for (String path : new String[]{"/admin/shipment-jobs/replay", "/admin/sync/9/retry"}) {
            mvc.perform(post(path).with(user("employee").roles("EMPLOYEE")).with(csrf())).andExpect(status().isForbidden());
            mvc.perform(post(path).with(user("admin").roles("EMPLOYEE", "ADMIN")).with(csrf())).andExpect(status().isOk());
        }
    }

    private MockMvc mvc(boolean training) throws Exception {
        var context = new org.springframework.web.context.support.AnnotationConfigWebApplicationContext();
        context.register(training ? TrainingConfiguration.class : SafeConfiguration.class);
        context.setServletContext(new org.springframework.mock.web.MockServletContext());
        context.refresh();
        return MockMvcBuilders.webAppContextSetup(context).apply(org.springframework.security.test.web.servlet.setup.SecurityMockMvcConfigurers.springSecurity()).build();
    }

    private static SecurityFilterChain security(HttpSecurity http, boolean training) {
        try {
            SecurityRules.authorize(http, training);
            return http.build();
        } catch (Exception e) { throw new IllegalStateException(e); }
    }

    @Configuration
    @EnableWebMvc
    @EnableWebSecurity
    static class TrainingConfiguration {
        @Bean RouteController routeController() { return new RouteController(); }
        @Bean SecurityFilterChain filterChain(HttpSecurity http) { return security(http, true); }
    }

    @Configuration
    @EnableWebMvc
    @EnableWebSecurity
    static class SafeConfiguration {
        @Bean RouteController routeController() { return new RouteController(); }
        @Bean SecurityFilterChain filterChain(HttpSecurity http) { return security(http, false); }
    }

    @RestController
    static class RouteController {
        @RequestMapping({"/actuator/mappings", "/admin/shipment-jobs/7/export", "/admin/shipment-jobs/replay", "/admin/sync/9", "/admin/sync/9/retry", "/admin/sync"})
        String ok() { return "ok"; }
    }
}
