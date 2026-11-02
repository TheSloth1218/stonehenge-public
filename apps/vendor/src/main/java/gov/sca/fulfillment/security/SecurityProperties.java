package gov.sca.fulfillment.security;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "sca.security")
public record SecurityProperties(
        String employeeGroup,
        String adminGroup,
        boolean developmentAuth
) {
}
