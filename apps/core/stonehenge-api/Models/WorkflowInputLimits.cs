namespace Stonehenge.Api.Models;

public static class WorkflowInputLimits
{
    public const long MaxRequestBodyBytes = 16 * 1024;
    public const int MaxPartRequests = 128;
    public const int MaxCompletions = 128;
    public const int MaxRequestIdLength = 128;
    public const int MaxStatusLength = 64;
    public const int MaxPriorityLength = 32;
    public const int MaxCompletionNoteLength = 2048;
    public const int MaxTrackingReferenceLength = 128;
}
