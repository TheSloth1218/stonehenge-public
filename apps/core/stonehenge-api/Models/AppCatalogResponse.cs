namespace Stonehenge.Api.Models;

public sealed record AppCatalogResponse(IReadOnlyList<StonehengeAppDescriptor> Apps);
