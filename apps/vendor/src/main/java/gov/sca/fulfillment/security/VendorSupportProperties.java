package gov.sca.fulfillment.security;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "vendor.support")
public record VendorSupportProperties(
        String command,
        String profileDirectory,
        String pluginDirectory,
        String mode
) {
}
