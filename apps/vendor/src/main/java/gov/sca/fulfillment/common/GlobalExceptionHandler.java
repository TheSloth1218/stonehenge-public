package gov.sca.fulfillment.common;

import gov.sca.fulfillment.sync.StonehengeApiException;
import org.springframework.ui.Model;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.ResponseStatus;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.web.bind.annotation.ControllerAdvice;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.UUID;

@ControllerAdvice
public class GlobalExceptionHandler {
    private static final Logger log = LoggerFactory.getLogger(GlobalExceptionHandler.class);

    @ExceptionHandler(AccessDeniedException.class)
    @ResponseStatus(HttpStatus.FORBIDDEN)
    public String accessDenied() {
        return "error/access-denied";
    }

    @ExceptionHandler(StonehengeApiException.class)
    @ResponseStatus(HttpStatus.BAD_GATEWAY)
    public String upstreamFailure(StonehengeApiException ex, Model model) {
        return expected(ex, model);
    }

    @ExceptionHandler({AppException.class, NotFoundException.class})
    public String expected(RuntimeException ex, Model model) {
        model.addAttribute("title", ex instanceof NotFoundException ? "Record not found" : "Action could not be completed");
        model.addAttribute("message", ex.getMessage());
        model.addAttribute("reference", UUID.randomUUID().toString());
        return "error/expected";
    }

    @ExceptionHandler(Exception.class)
    @ResponseStatus(HttpStatus.INTERNAL_SERVER_ERROR)
    public String unexpected(Exception ex, Model model) {
        String reference = UUID.randomUUID().toString();
        log.error("Unhandled application error reference={}", reference, ex);
        model.addAttribute("title", "Unexpected application error");
        model.addAttribute("message", "The request could not be completed. Use the reference identifier for operational follow-up.");
        model.addAttribute("reference", reference);
        return "error/expected";
    }
}
