package gov.sca.fulfillment;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class ScaPartsFulfillmentApplication {
    public static void main(String[] args) {
        SpringApplication.run(ScaPartsFulfillmentApplication.class, args);
    }
}
