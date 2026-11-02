package gov.sca.fulfillment.fulfillment;

import com.microsoft.playwright.Browser;
import com.microsoft.playwright.Page;
import com.microsoft.playwright.Playwright;
import org.junit.jupiter.api.Assumptions;
import org.junit.jupiter.api.Test;

import static com.microsoft.playwright.assertions.PlaywrightAssertions.assertThat;

class FulfillmentBrowserWorkflowTest {
    @Test
    void requestFulfillmentWorkflow() {
        Assumptions.assumeTrue(Boolean.parseBoolean(System.getenv().getOrDefault("RUN_PLAYWRIGHT", "false")),
                "Set RUN_PLAYWRIGHT=true with the app running at PLAYWRIGHT_BASE_URL to run browser workflow tests.");

        String baseUrl = System.getenv().getOrDefault("PLAYWRIGHT_BASE_URL", "http://127.0.0.1:8089");
        try (Playwright playwright = Playwright.create();
             Browser browser = playwright.chromium().launch()) {
            Page page = browser.newPage();
            page.navigate(baseUrl + "/requests");
            assertThat(page.getByRole(com.microsoft.playwright.options.AriaRole.HEADING,
                    new Page.GetByRoleOptions().setName("Maintenance Request Queue"))).isVisible();
            page.getByRole(com.microsoft.playwright.options.AriaRole.LINK,
                    new Page.GetByRoleOptions().setName("Open Request")).first().click();
            assertThat(page.getByText("Parts Search")).isVisible();
        }
    }
}
