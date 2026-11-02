package gov.sca.fulfillment.security;

import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.security.core.AuthenticationException;
import org.springframework.security.oauth2.core.OAuth2AuthenticationException;
import org.springframework.security.web.authentication.AuthenticationFailureHandler;
import org.springframework.stereotype.Component;
import org.springframework.web.util.UriComponentsBuilder;

import java.io.IOException;
import java.util.UUID;

@Component
class OidcAuthenticationFailureHandler implements AuthenticationFailureHandler {
    private static final Logger log = LoggerFactory.getLogger(OidcAuthenticationFailureHandler.class);

    @Override
    public void onAuthenticationFailure(HttpServletRequest request, HttpServletResponse response,
                                        AuthenticationException failure) throws IOException, ServletException {
        String reference = UUID.randomUUID().toString();
        String code = "authentication_failed";
        String description = failure.getClass().getSimpleName();
        if (failure instanceof OAuth2AuthenticationException oauth) {
            code = safe(oauth.getError().getErrorCode());
            description = safe(oauth.getError().getDescription());
        }
        log.warn("OIDC login failed reference={} code={} detail={}", reference, code, description);
        response.sendRedirect(UriComponentsBuilder.fromPath("/access-denied")
                .queryParam("reference", reference)
                .queryParam("reason", "oidc")
                .build()
                .encode()
                .toUriString());
    }

    private String safe(String value) {
        if (value == null || value.isBlank()) {
            return "unavailable";
        }
        return value.replaceAll("[\\r\\n\\t]", " ").substring(0, Math.min(value.length(), 240));
    }
}
