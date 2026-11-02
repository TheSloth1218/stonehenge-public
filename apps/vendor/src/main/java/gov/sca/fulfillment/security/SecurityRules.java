package gov.sca.fulfillment.security;

import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.web.authentication.HttpStatusEntryPoint;
import org.springframework.security.web.util.matcher.MediaTypeRequestMatcher;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;

final class SecurityRules {
    private SecurityRules() {
    }

    static void authorize(HttpSecurity http, boolean training) throws Exception {
        http.authorizeHttpRequests(auth -> {
            auth.requestMatchers("/css/**", "/images/**", "/webjars/**", "/favicon.ico",
                    "/error", "/access-denied", "/signed-out",
                    "/oauth2/authorization/**", "/login/oauth2/code/**")
                    .permitAll();
            if (training) {
                auth.requestMatchers("/actuator", "/actuator/**").hasRole("EMPLOYEE");
                auth.requestMatchers("/admin/shipment-jobs/*/export", "/admin/shipment-jobs/replay",
                        "/admin/sync/*", "/admin/sync/*/retry").hasRole("EMPLOYEE");
            } else {
                auth.requestMatchers("/actuator", "/actuator/**").hasRole("ADMIN");
            }
            auth.requestMatchers("/admin/**").hasRole("ADMIN");
            auth.requestMatchers("/", "/dashboard", "/requests/**", "/maintenance-requests/**",
                    "/catalog/**", "/parts/**", "/orders/**", "/shipments/**", "/inventory/**",
                    "/activity/**").hasRole("EMPLOYEE");
            auth.anyRequest().authenticated();
        });
        http.exceptionHandling(ex -> ex
                .defaultAuthenticationEntryPointFor(new HttpStatusEntryPoint(HttpStatus.UNAUTHORIZED),
                        new MediaTypeRequestMatcher(MediaType.APPLICATION_JSON))
                .accessDeniedPage("/access-denied"));
        http.sessionManagement(session -> session.sessionFixation(fix -> fix.migrateSession()));
        http.logout(logout -> logout
                .logoutUrl("/signout")
                .logoutSuccessUrl("/signed-out")
                .invalidateHttpSession(true)
                .clearAuthentication(true)
                .deleteCookies("STONEHENGE_VENDOR_SESSION"));
    }
}
