package gov.sca.fulfillment.config;

import gov.sca.fulfillment.sync.StonehengeApiProperties;
import gov.sca.fulfillment.security.SecurityProperties;
import gov.sca.fulfillment.security.TrainingProperties;
import gov.sca.fulfillment.security.VendorSupportProperties;
import gov.sca.fulfillment.shipping.jobs.ShipmentJobProperties;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Configuration;

@Configuration
@EnableConfigurationProperties({StonehengeApiProperties.class, SecurityProperties.class,
        TrainingProperties.class, ShipmentJobProperties.class, VendorSupportProperties.class})
public class AppConfig {
}
