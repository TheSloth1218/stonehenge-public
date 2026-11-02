package gov.sca.fulfillment.maintenance;

import gov.sca.fulfillment.catalog.PartRepository;
import gov.sca.fulfillment.fulfillment.FulfillmentOrder;
import gov.sca.fulfillment.fulfillment.FulfillmentOrderRepository;
import gov.sca.fulfillment.fulfillment.FulfillmentService;
import gov.sca.fulfillment.inventory.InventoryRepository;
import gov.sca.fulfillment.sync.VendorApiSyncService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;

import java.util.Optional;

import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.redirectedUrl;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

class MaintenanceControllerBindingTest {
    private FulfillmentService service;
    private FulfillmentOrderRepository orders;
    private MockMvc mvc;

    @BeforeEach
    void setUp() {
        service = mock(FulfillmentService.class);
        orders = mock(FulfillmentOrderRepository.class);
        MaintenanceController controller = new MaintenanceController(
                mock(MaintenanceRequestRepository.class), orders, mock(PartRepository.class),
                mock(InventoryRepository.class), service, mock(VendorApiSyncService.class));
        mvc = MockMvcBuilders.standaloneSetup(controller).build();

        FulfillmentOrder order = new FulfillmentOrder();
        order.setId(42L);
        when(orders.findByMaintenanceRequestId(7L)).thenReturn(Optional.of(order));
    }

    @Test
    void removeItemReturnsToRequestWorkspace() throws Exception {
        mvc.perform(post("/requests/7/orders/42/items/9/remove"))
                .andExpect(status().is3xxRedirection())
                .andExpect(redirectedUrl("/requests/7"));

        verify(service).removeItem(42L, 9L);
    }

    @Test
    void clearOrderReturnsToRequestWorkspace() throws Exception {
        mvc.perform(post("/requests/7/orders/42/clear"))
                .andExpect(status().is3xxRedirection())
                .andExpect(redirectedUrl("/requests/7"));

        verify(service).clearItems(42L);
    }
}
