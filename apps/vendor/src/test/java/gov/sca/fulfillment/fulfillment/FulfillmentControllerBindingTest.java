package gov.sca.fulfillment.fulfillment;

import gov.sca.fulfillment.shipping.ShipmentProgressService;
import gov.sca.fulfillment.inventory.InventoryRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.redirectedUrl;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

class FulfillmentControllerBindingTest {
    private FulfillmentService service;
    private MockMvc mvc;

    @BeforeEach
    void setUp() {
        service = mock(FulfillmentService.class);
        FulfillmentController controller = new FulfillmentController(
                mock(FulfillmentOrderRepository.class), service, mock(ShipmentProgressService.class),
                mock(InventoryRepository.class));
        mvc = MockMvcBuilders.standaloneSetup(controller).build();
    }

    @Test
    void shipmentFormBindsAndRedirectsToStatus() throws Exception {
        mvc.perform(post("/orders/42/ship")
                        .param("carrier", "SCA Motor Pool")
                        .param("shippingMethod", "Truck Transport")
                        .param("destinationLabel", "Stonehenge Engineering Maintenance Bay")
                        .param("notes", "Packed")
                        .param("confirmed", "true")
                        .param("externalRequestId", "PR-2001"))
                .andExpect(status().is3xxRedirection())
                .andExpect(redirectedUrl("/orders/42/shipping-status"));

        ArgumentCaptor<ShipOrderForm> form = ArgumentCaptor.forClass(ShipOrderForm.class);
        verify(service).shipAndSync(org.mockito.ArgumentMatchers.eq(42L), form.capture());
        assertThat(form.getValue().carrier()).isEqualTo("SCA Motor Pool");
        assertThat(form.getValue().shippingMethod()).isEqualTo("Truck Transport");
        assertThat(form.getValue().confirmed()).isTrue();
        assertThat(form.getValue().externalRequestId()).isEqualTo("PR-2001");
    }

    @Test
    void removePostsDirectlyToService() throws Exception {
        mvc.perform(post("/orders/42/items/9/remove"))
                .andExpect(status().is3xxRedirection())
                .andExpect(redirectedUrl("/orders/42/review"));

        verify(service).removeItem(42L, 9L);
    }
}
