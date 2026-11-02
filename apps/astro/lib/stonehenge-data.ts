export type AlertLevel = "nominal" | "advisory" | "watch" | "impact";
export type InterceptStatus = "pending" | "intercepting" | "intercepted";

export type InterceptRecord = {
  id: string;
  fragmentId: string;
  status: InterceptStatus;
  interceptedAt: string;
  locationName: string;
  x: number;
  y: number;
  battery: string;
  confidence: number;
};

export type LocationMarker = {
  name: string;
  code: string;
  x: number;
  y: number;
  kind: "battery" | "city" | "relay" | "restricted";
};

export const theaterMarkers: LocationMarker[] = [
  { name: "Stonehenge", code: "STN", x: 334, y: 350, kind: "battery" },
  { name: "San Salvacion", code: "SSV", x: 420, y: 292, kind: "city" },
  { name: "Farbanti", code: "FAR", x: 158, y: 260, kind: "city" },
  { name: "Rigley Air Base", code: "RAB", x: 606, y: 304, kind: "relay" },
  { name: "Los Canas", code: "LCA", x: 586, y: 418, kind: "city" },
  { name: "Newfield", code: "NFD", x: 800, y: 224, kind: "relay" },
  { name: "North Point", code: "NPT", x: 812, y: 178, kind: "city" }
];

export const interceptRecords: InterceptRecord[] = [
  {
    id: "U-1482",
    fragmentId: "LYRE-1482",
    status: "intercepted",
    interceptedAt: "1999-07-08T14:05:32",
    locationName: "Newfield outer box",
    x: 917,
    y: 280,
    battery: "STN-04",
    confidence: 0.92
  },
  {
    id: "U-1771",
    fragmentId: "BELL-1771",
    status: "intercepted",
    interceptedAt: "1999-07-08T14:09:18",
    locationName: "San Salvacion civil perimeter",
    x: 498,
    y: 359,
    battery: "STN-02",
    confidence: 0.81
  },
  {
    id: "U-1904",
    fragmentId: "EMBER-1904",
    status: "pending",
    interceptedAt: "1999-07-08T14:12:44",
    locationName: "North Point terminal gate",
    x: 967,
    y: 216,
    battery: "STN-04",
    confidence: 0.67
  },
  {
    id: "U-2016",
    fragmentId: "HARP-2016",
    status: "intercepting",
    interceptedAt: "1999-07-08T14:18:03",
    locationName: "Los Canas southern approach",
    x: 669,
    y: 520,
    battery: "STN-06",
    confidence: 0.74
  }
];

export const publicNotices = [
  {
    time: "14:12 IUN",
    title: "Southern shelter network remains active",
    body: "Civil-defense offices report clear access to municipal shelter routes in San Salvacion and the adjacent lowlands.",
    level: "advisory" as const
  },
  {
    time: "13:48 IUN",
    title: "Stonehenge rail traverse completed",
    body: "Battery 04 returned to calibration hold after a scheduled azimuth test. No public action is required.",
    level: "nominal" as const
  },
  {
    time: "13:09 IUN",
    title: "Cape Rainy maritime box closed",
    body: "Harbor authority has closed the marked exclusion grid while fragment U-1904 remains in terminal review.",
    level: "watch" as const
  }
];

export const archiveReports = [
  "Ulysses Fragment Registry, civilian digest 19",
  "Stonehenge Program: rail cannon alignment overview",
  "San Salvacion construction site overview",
  "Stonehenge terminal-fragment readiness notice"
];
