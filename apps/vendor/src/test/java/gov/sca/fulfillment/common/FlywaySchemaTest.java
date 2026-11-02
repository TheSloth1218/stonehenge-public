package gov.sca.fulfillment.common;

import gov.sca.fulfillment.catalog.PartRepository;
import gov.sca.fulfillment.maintenance.MaintenanceRequestRepository;
import gov.sca.fulfillment.fulfillment.FulfillmentOrderRepository;
import gov.sca.fulfillment.users.AppUserRepository;
import gov.sca.fulfillment.users.UserRole;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;

import static org.assertj.core.api.Assertions.assertThat;

class FlywaySchemaTest extends PostgresIntegrationTest {
    @Autowired PartRepository parts;
    @Autowired MaintenanceRequestRepository requests;
    @Autowired FulfillmentOrderRepository orders;
    @Autowired AppUserRepository users;

    @Test
    void migrationsLoadSchemaAndSeedData() {
        assertThat(parts.count()).isGreaterThanOrEqualTo(12);
        assertThat(requests.count()).isZero();
        assertThat(orders.count()).isZero();
        assertThat(users.findByUsername("operator").orElseThrow().getRole()).isEqualTo(UserRole.EMPLOYEE);
        assertThat(users.findByUsername("admin").orElseThrow().getRole()).isEqualTo(UserRole.ADMIN);
    }
}
