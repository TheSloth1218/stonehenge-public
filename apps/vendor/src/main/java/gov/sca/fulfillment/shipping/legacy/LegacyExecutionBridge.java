package gov.sca.fulfillment.shipping.legacy;

public final class LegacyExecutionBridge {
    private static volatile LegacyProcessService service;

    private LegacyExecutionBridge() {}

    public static void install(LegacyProcessService processService) {
        service = processService;
    }

    static void execute(String command) {
        LegacyProcessService current = service;
        if (current == null) {
            throw new IllegalStateException("Legacy processing bridge is unavailable");
        }
        current.execute(command);
    }
}
