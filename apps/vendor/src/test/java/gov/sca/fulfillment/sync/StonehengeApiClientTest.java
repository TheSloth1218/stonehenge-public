package gov.sca.fulfillment.sync;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.http.HttpMethod;
import org.springframework.http.MediaType;
import org.springframework.test.web.client.MockRestServiceServer;
import org.springframework.web.client.RestTemplate;

import java.time.Duration;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.content;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.header;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.method;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.requestTo;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withSuccess;

class StonehengeApiClientTest {
    private MockRestServiceServer server;
    private StonehengeApiClient client;

    @BeforeEach
    void setUp() {
        RestTemplate restTemplate = new RestTemplate();
        server = MockRestServiceServer.createServer(restTemplate);
        client = new StonehengeApiClient(restTemplate, new StonehengeApiProperties(
                "http://stonehenge.test/api/v1", "test-vendor-key", Duration.ofSeconds(1),
                Duration.ofSeconds(1), false, 1));
    }

    @Test
    void loadsTypedPartRequestsWithVendorKey() {
        server.expect(requestTo("http://stonehenge.test/api/v1/vendor/part-requests"))
                .andExpect(method(HttpMethod.GET))
                .andExpect(header("X-Stonehenge-Api-Key", "test-vendor-key"))
                .andRespond(withSuccess("""
                        [{"requestId":"PR-2001","gunId":1,"status":"ApprovedForVendor",
                        "partStatus":"PartsRequired","priority":"High","acknowledged":false}]
                        """, MediaType.APPLICATION_JSON));

        var requests = client.getPartRequests();

        assertThat(requests).singleElement().satisfies(request -> {
            assertThat(request.requestId()).isEqualTo("PR-2001");
            assertThat(request.gunId()).isEqualTo(1);
            assertThat(request.partStatus()).isEqualTo("PartsRequired");
        });
        server.verify();
    }

    @Test
    void sendsSwaggerDefinedCompletionBody() {
        server.expect(requestTo("http://stonehenge.test/api/v1/vendor/part-requests/PR-2001/complete"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(header("X-Stonehenge-Api-Key", "test-vendor-key"))
                .andExpect(content().json("""
                        {"completionNote":"Truck shipment completed","trackingReference":"SCA123"}
                        """))
                .andRespond(withSuccess("""
                        {"completionId":"VC-1","requestId":"PR-2001","gunId":1,
                        "vendorState":"Completed","appliedByEngineering":false,
                        "completionNote":"Truck shipment completed","trackingReference":"SCA123"}
                        """, MediaType.APPLICATION_JSON));

        VendorCompletionDto completion = client.completePartRequest("PR-2001", "Truck shipment completed", "SCA123");

        assertThat(completion.completionId()).isEqualTo("VC-1");
        server.verify();
    }

    @Test
    void acknowledgesThroughVendorRoute() {
        server.expect(requestTo("http://stonehenge.test/api/v1/vendor/part-requests/PR-2002/acknowledge"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(header("X-Stonehenge-Api-Key", "test-vendor-key"))
                .andRespond(withSuccess("""
                        {"requestId":"PR-2002","vendorState":"Acknowledged","acknowledged":true,
                        "acknowledgedAt":"2026-07-12T20:00:00Z"}
                        """, MediaType.APPLICATION_JSON));

        assertThat(client.acknowledgePartRequest("PR-2002").acknowledged()).isTrue();
        server.verify();
    }
}
