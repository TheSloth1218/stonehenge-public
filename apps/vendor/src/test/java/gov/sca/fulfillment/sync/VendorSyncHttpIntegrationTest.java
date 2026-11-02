package gov.sca.fulfillment.sync;

import gov.sca.fulfillment.common.GlobalExceptionHandler;
import gov.sca.fulfillment.maintenance.MaintenanceController;
import gov.sca.fulfillment.maintenance.MaintenanceRequestRepository;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.time.Instant;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest(properties = {"stonehenge.api.polling-attempts=1", "sca.shipment.worker.enabled=false"})
@ActiveProfiles("test")
@Testcontainers
class VendorSyncHttpIntegrationTest {
    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("stonehenge_vendor_sync_test")
            .withUsername("stonehenge_vendor")
            .withPassword("change-me-local-dev");

    @DynamicPropertySource
    static void databaseProperties(DynamicPropertyRegistry registry) {
        registry.add("spring.datasource.url", POSTGRES::getJdbcUrl);
        registry.add("spring.datasource.username", POSTGRES::getUsername);
        registry.add("spring.datasource.password", POSTGRES::getPassword);
        registry.add("stonehenge.api.api-key", () -> "test-vendor-key");
    }

    @Autowired MaintenanceController controller;
    @Autowired MaintenanceRequestRepository requests;
    @MockBean StonehengeApiClient api;
    private MockMvc mvc;
    private String requestId;

    @BeforeEach
    void setUp() {
        requestId = "QA-SYNC-" + UUID.randomUUID();
        // Real service, JPA repository and PostgreSQL transactions; only the upstream API is mocked.
        // No test-managed transaction: each assertion observes committed state after the HTTP action.
        mvc = MockMvcBuilders.standaloneSetup(controller)
                .setControllerAdvice(new GlobalExceptionHandler()).build();
        when(api.getPartRequests()).thenReturn(List.of(request(requestId, "ApprovedForVendor")));
        when(api.getVendorCompletions()).thenReturn(List.of());
        when(api.getGunStatus()).thenReturn(List.of(gun()));
    }

    @AfterEach
    void cleanUp() {
        requests.findByExternalRequestId(requestId).ifPresent(requests::delete);
        requests.findByExternalRequestId(requestId + "-second").ifPresent(requests::delete);
    }

    @Test
    void successfulAndRepeatedSyncCommitOneRequest() throws Exception {
        mvc.perform(post("/requests/sync")).andExpect(status().isFound())
                .andExpect(redirectedUrl("/requests"));
        var first = requests.findByExternalRequestId(requestId).orElseThrow();
        long count = requests.count();

        mvc.perform(post("/requests/sync")).andExpect(status().isFound());
        var second = requests.findByExternalRequestId(requestId).orElseThrow();
        assertThat(second.getId()).isEqualTo(first.getId());
        assertThat(second.getRemoteStatus()).isEqualTo("ApprovedForVendor");
        assertThat(requests.count()).isEqualTo(count);
    }

    @Test
    void malformedGunResponseReturnsBadGatewayAndImportsNothing() throws Exception {
        when(api.getGunStatus()).thenReturn(Arrays.asList(gun(), null));
        mvc.perform(post("/requests/sync")).andExpect(status().isBadGateway())
                .andExpect(view().name("error/expected"));
        assertThat(requests.findByExternalRequestId(requestId)).isEmpty();
    }

    @Test
    void upstreamFailureReturnsBadGatewayAndImportsNothing() throws Exception {
        when(api.getPartRequests()).thenThrow(new StonehengeApiException("Upstream unavailable"));
        mvc.perform(post("/requests/sync")).andExpect(status().isBadGateway());
        assertThat(requests.findByExternalRequestId(requestId)).isEmpty();
    }

    @Test
    void databaseFailureRollsBackEarlierInserts() throws Exception {
        when(api.getPartRequests()).thenReturn(List.of(request(requestId, "ApprovedForVendor"),
                request(requestId + "-second", "x".repeat(10000))));
        mvc.perform(post("/requests/sync")).andExpect(status().isInternalServerError());
        assertThat(requests.findByExternalRequestId(requestId)).isEmpty();
        assertThat(requests.findByExternalRequestId(requestId + "-second")).isEmpty();
    }

    private static VendorPartRequestDto request(String id, String status) {
        Instant now = Instant.parse("2026-09-19T00:00:00Z");
        return new VendorPartRequestDto(id, 1, status, "Ready", "NORMAL", now, now,
                false, Map.of(), Map.of());
    }

    private static VendorGunStatusDto gun() {
        return new VendorGunStatusDto(1, "AUTO", 0, "Ready", null, Map.of());
    }
}
