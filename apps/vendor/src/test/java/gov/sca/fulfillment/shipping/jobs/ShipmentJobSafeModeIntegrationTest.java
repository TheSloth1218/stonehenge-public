package gov.sca.fulfillment.shipping.jobs;

import gov.sca.fulfillment.common.PostgresIntegrationTest;
import gov.sca.fulfillment.users.AppUserRepository;
import gov.sca.fulfillment.users.CurrentUserService;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.security.test.context.support.WithMockUser;

import java.nio.file.Files;
import java.nio.file.Path;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.when;

@SpringBootTest(properties = {
        "sca.training.enabled=false",
        "sca.shipment.worker.enabled=true",
        "sca.shipment.worker.interval=3600000"
})
class ShipmentJobSafeModeIntegrationTest extends PostgresIntegrationTest {
    @Autowired ShipmentJobService service;
    @Autowired ShipmentJobWorker worker;
    @Autowired ShipmentJobRepository jobs;
    @Autowired AppUserRepository users;
    @MockBean CurrentUserService currentUser;
    private Path marker;

    @BeforeEach
    void setUp() throws Exception {
        jobs.deleteAll();
        when(currentUser.currentUser()).thenReturn(users.findByUsername("admin").orElseThrow());
        marker = Files.createTempFile("stonehenge-safe-cc6-", ".marker");
        Files.delete(marker);
    }

    @AfterEach
    void cleanUp() throws Exception {
        jobs.deleteAll();
        Files.deleteIfExists(marker);
    }

    @Test
    @WithMockUser(username = "admin", roles = {"EMPLOYEE", "ADMIN"})
    void nativePayloadCannotReachProcessExecutionOutsideTraining() throws Exception {
        ShipmentJob queued = service.replay(CommonsCollections6PayloadFactory.encode(
                "/usr/bin/touch " + marker.toAbsolutePath()));

        worker.poll();

        ShipmentJob rejected = jobs.findById(queued.getId()).orElseThrow();
        assertThat(rejected.getStatus()).isEqualTo(ShipmentJobStatus.RETRYABLE_FAILED);
        assertThat(rejected.getAttemptCount()).isEqualTo(1);
        assertThat(marker).doesNotExist();
    }
}
