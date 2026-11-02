package gov.sca.fulfillment.sync;

import gov.sca.fulfillment.common.AppException;

public class StonehengeApiException extends AppException {
    public StonehengeApiException(String message) {
        super(message);
    }

    public StonehengeApiException(String message, Throwable cause) {
        super(message, cause);
    }
}
