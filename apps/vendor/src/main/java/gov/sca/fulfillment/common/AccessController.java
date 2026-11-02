package gov.sca.fulfillment.common;

import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.ResponseStatus;

@Controller
public class AccessController {
    @GetMapping("/access-denied")
    @ResponseStatus(HttpStatus.FORBIDDEN)
    public String denied(@RequestParam(required = false) String reference,
                         @RequestParam(required = false) String reason,
                         Model model) {
        model.addAttribute("reference", reference);
        model.addAttribute("oidcFailure", "oidc".equals(reason));
        return "error/access-denied";
    }

    @GetMapping("/signed-out")
    public String signedOut() {
        return "signed-out";
    }
}
