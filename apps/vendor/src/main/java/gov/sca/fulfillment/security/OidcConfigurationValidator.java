package gov.sca.fulfillment.security;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;

import java.net.URI;

@Component
@Profile("!dev & !test")
final class OidcConfigurationValidator {
    OidcConfigurationValidator(SecurityProperties security,
                               @Value("${spring.security.oauth2.client.registration.dex.client-id:}") String clientId,
                               @Value("${spring.security.oauth2.client.registration.dex.client-secret:}") String clientSecret,
                               @Value("${spring.security.oauth2.client.provider.dex.issuer-uri:}") String issuerUri,
                               @Value("${app.public-origin:}") String publicOrigin,
                               @Value("${app.auth-session-secret:}") String authSessionSecret) {
        requireText(clientId, "OIDC_CLIENT_ID must be configured for Dex authentication");
        requireText(clientSecret, "OIDC_CLIENT_SECRET must be configured for Dex authentication");
        requireHttpsOrigin(issuerUri, "OIDC_ISSUER");
        requireHttpsOrigin(publicOrigin, "APP_PUBLIC_ORIGIN");
        requireText(authSessionSecret, "AUTH_SESSION_SECRET must be configured");
        if (authSessionSecret.length() < 32) {
            throw new IllegalStateException("AUTH_SESSION_SECRET must contain at least 32 characters");
        }
        requireText(security.employeeGroup(), "SCA_EMPLOYEE_GROUP must be configured");
        requireText(security.adminGroup(), "SCA_ADMIN_GROUP must be configured");
        if (security.employeeGroup().equals(security.adminGroup())) {
            throw new IllegalStateException("Dex employee and admin groups must be different");
        }
    }

    private static void requireHttpsOrigin(String value, String name) {
        requireText(value, name + " must be configured");
        URI uri;
        try {
            uri = URI.create(value);
        } catch (IllegalArgumentException error) {
            throw new IllegalStateException(name + " must be a valid HTTPS URL", error);
        }
        if (!"https".equalsIgnoreCase(uri.getScheme()) || uri.getHost() == null
                || uri.getUserInfo() != null || uri.getQuery() != null || uri.getFragment() != null) {
            throw new IllegalStateException(name + " must be an HTTPS origin");
        }
        if ("APP_PUBLIC_ORIGIN".equals(name)
                && uri.getPath() != null && !uri.getPath().isEmpty() && !"/".equals(uri.getPath())) {
            throw new IllegalStateException(name + " must not contain a path");
        }
    }

    private static void requireText(String value, String message) {
        if (value == null || value.isBlank()) {
            throw new IllegalStateException(message);
        }
    }
}
