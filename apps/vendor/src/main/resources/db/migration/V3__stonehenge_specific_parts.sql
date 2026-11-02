update parts set
    sku = 'STN-COIL-4100',
    name = 'Accelerator Coil Segment A4',
    description = 'Replaceable coil segment for the Stonehenge kinetic accelerator barrel. Used when engineering reports coil insulation wear or uneven charge response.',
    category = 'Accelerator Coils',
    manufacturer = 'FCU Strategic Systems Depot',
    compatible_asset_class = 'Stonehenge Railgun Barrel',
    active = true,
    updated_at = now()
where sku = 'SCA-AZ-4100';

update parts set
    sku = 'STN-GEAR-2200',
    name = 'Elevation Gearbox Motor Set',
    description = 'Motor and reduction gearbox set for heavy elevation adjustment on the Stonehenge gun cradle.',
    category = 'Traverse/Elevation Drive',
    manufacturer = 'Osea Heavy Motion Works',
    compatible_asset_class = 'Gun Mount Drive',
    active = true,
    updated_at = now()
where sku = 'SCA-EL-2200';

update parts set
    sku = 'STN-FUSE-1204',
    name = 'Pulse Bank Isolation Fuse',
    description = 'High-current fuse module used to isolate a pulsed-power bank during maintenance service.',
    category = 'Pulsed Power & Fusing',
    manufacturer = 'North Point Power Components',
    compatible_asset_class = 'Pulsed Power Room',
    active = true,
    updated_at = now()
where sku = 'SCA-SR-1204';

update parts set
    sku = 'STN-PANEL-8008',
    name = 'Local Interlock Panel Module',
    description = 'Cabinet panel module for safe maintenance interlocks, permissive display, and local service status.',
    category = 'Fire-Control Consoles',
    manufacturer = 'San Salvacion Instrumentation',
    compatible_asset_class = 'Control Gallery',
    active = true,
    updated_at = now()
where sku = 'SCA-IO-8008';

update parts set
    sku = 'STN-LUBE-3310',
    name = 'Rail Contact Lubricant Pack',
    description = 'Approved lubricant and applicator set for barrel rail contact surfaces and sliding service points.',
    category = 'Rail/Barrel Service',
    manufacturer = 'Usea Ordnance Materials Lab',
    compatible_asset_class = 'Rail Contact Assembly',
    active = true,
    updated_at = now()
where sku = 'SCA-HY-3310';

update parts set
    sku = 'STN-COOL-0900',
    name = 'Barrel Cooling Hose Manifold Kit',
    description = 'Pre-cut coolant hose and manifold service kit for thermal management around the accelerator barrel.',
    category = 'Cooling/Lubrication',
    manufacturer = 'Aster Fluidics',
    compatible_asset_class = 'Barrel Cooling Loop',
    active = true,
    updated_at = now()
where sku = 'SCA-CH-0900';

update parts set
    sku = 'STN-SENS-7712',
    name = 'Bore Alignment Encoder Kit',
    description = 'Encoder, coupler, and service brackets for bore alignment feedback on traverse and elevation assemblies.',
    category = 'Sensors & Alignment',
    manufacturer = 'Orbital Instrument',
    compatible_asset_class = 'Gun Mount Drive',
    active = true,
    updated_at = now()
where sku = 'SCA-EN-7712';

update parts set
    sku = 'STN-PWR-6200',
    name = 'Pulse Distribution Breaker Module',
    description = 'Fused distribution module for non-energetic maintenance replacement in the pulsed-power support cabinet.',
    category = 'Pulsed Power & Fusing',
    manufacturer = 'Kepler Power',
    compatible_asset_class = 'Pulsed Power Cabinet',
    active = true,
    updated_at = now()
where sku = 'SCA-PD-6200';

update parts set
    sku = 'STN-MOUNT-4400',
    name = 'Cradle Fastener & Shim Kit',
    description = 'Approved fasteners, shims, and lock plates for service work on gun cradle covers and maintenance panels.',
    category = 'Structural Mounting',
    manufacturer = 'Summit Fabrication',
    compatible_asset_class = 'Gun Cradle',
    active = true,
    updated_at = now()
where sku = 'SCA-MF-4400';

update parts set
    sku = 'STN-CABLE-1500',
    name = 'Shielded Control Trunk Cable',
    description = 'Ruggedized shielded trunk cable for monitor, interlock, and local panel connections in the control gallery.',
    category = 'Harnesses & Cabling',
    manufacturer = 'Kepler Power',
    compatible_asset_class = 'Control Gallery',
    active = true,
    updated_at = now()
where sku = 'SCA-NW-1500';

update parts set
    sku = 'STN-FILT-2280',
    name = 'Coolant Return Filter Element',
    description = 'Replacement filter element for barrel cooling return loops and service carts.',
    category = 'Cooling/Lubrication',
    manufacturer = 'Aster Fluidics',
    compatible_asset_class = 'Barrel Cooling Loop',
    active = true,
    updated_at = now()
where sku = 'SCA-FL-2280';

update parts set
    sku = 'STN-SW-5100',
    name = 'Retired Cradle Limit Switch',
    description = 'Retired limit switch retained for historical orders only. Do not add to new fulfillment work unless reactivated by logistics administration.',
    category = 'Fire-Control Consoles',
    manufacturer = 'Orbital Instrument',
    compatible_asset_class = 'Gun Cradle',
    active = false,
    updated_at = now()
where sku = 'SCA-LK-5100';

insert into parts (sku, name, description, category, manufacturer, compatible_asset_class, image_path, active)
values
('STN-COIL-4200', 'Accelerator Coil Segment B7', 'Alternate barrel coil segment used for mid-run accelerator service windows.', 'Accelerator Coils', 'FCU Strategic Systems Depot', 'Stonehenge Railgun Barrel', '/css/part-placeholder.svg', true),
('STN-BARREL-3000', 'Sacrificial Rail Liner Set', 'Replaceable rail liner set for barrel wear surfaces exposed to projectile acceleration loads.', 'Rail/Barrel Service', 'Usea Ordnance Materials Lab', 'Rail Contact Assembly', '/css/part-placeholder.svg', true),
('STN-MON-7000', 'Service Gallery Monitor', 'Rugged local monitor for maintenance status, checklist display, and safe panel diagnostics.', 'Fire-Control Consoles', 'San Salvacion Instrumentation', 'Control Gallery', '/css/part-placeholder.svg', true),
('STN-SW-7300', 'Panel Key Switch Assembly', 'Two-position keyed service switch assembly for local panel replacement.', 'Fire-Control Consoles', 'San Salvacion Instrumentation', 'Control Gallery', '/css/part-placeholder.svg', true),
('STN-CABLE-1800', 'Coil Sensor Harness', 'Shielded branch harness for coil temperature and continuity sensor runs.', 'Harnesses & Cabling', 'Kepler Power', 'Stonehenge Railgun Barrel', '/css/part-placeholder.svg', true),
('STN-MOTOR-2500', 'Traverse Slew Motor Brush Kit', 'Service brush and seal kit for traverse slew motor maintenance.', 'Traverse/Elevation Drive', 'Osea Heavy Motion Works', 'Gun Mount Drive', '/css/part-placeholder.svg', true),
('STN-OIL-3320', 'Cradle Gear Lubricant Canister', 'Low-temperature lubricant canister for cradle gearing and exposed service gearboxes.', 'Cooling/Lubrication', 'Usea Ordnance Materials Lab', 'Gun Mount Drive', '/css/part-placeholder.svg', true),
('STN-SENS-7800', 'Rail Temperature Sensor Pair', 'Matched sensor pair for barrel rail temperature monitoring after engineering maintenance reports.', 'Sensors & Alignment', 'Orbital Instrument', 'Stonehenge Railgun Barrel', '/css/part-placeholder.svg', true)
on conflict (sku) do nothing;

insert into inventory (part_id, warehouse_location, bin_location, quantity_on_hand, quantity_reserved, reorder_level)
select p.id, 'Stonehenge Logistics Cage B', 'B-' || lpad(p.id::text, 2, '0'),
       case p.sku
           when 'STN-COIL-4200' then 3
           when 'STN-BARREL-3000' then 2
           when 'STN-MON-7000' then 5
           when 'STN-SW-7300' then 9
           when 'STN-CABLE-1800' then 8
           when 'STN-MOTOR-2500' then 4
           when 'STN-OIL-3320' then 16
           when 'STN-SENS-7800' then 6
           else 1
       end,
       0,
       case when p.sku in ('STN-BARREL-3000', 'STN-COIL-4200') then 2 else 3 end
from parts p
where p.sku in ('STN-COIL-4200', 'STN-BARREL-3000', 'STN-MON-7000', 'STN-SW-7300', 'STN-CABLE-1800', 'STN-MOTOR-2500', 'STN-OIL-3320', 'STN-SENS-7800')
  and not exists (select 1 from inventory i where i.part_id = p.id);

update inventory i
set warehouse_location = 'Stonehenge Logistics Cage A',
    bin_location = 'STN-' || lpad(i.id::text, 2, '0'),
    quantity_on_hand = case p.sku
        when 'STN-COIL-4100' then 4
        when 'STN-GEAR-2200' then 3
        when 'STN-FUSE-1204' then 14
        when 'STN-PANEL-8008' then 7
        when 'STN-LUBE-3310' then 18
        when 'STN-COOL-0900' then 2
        when 'STN-SENS-7712' then 5
        when 'STN-PWR-6200' then 6
        when 'STN-MOUNT-4400' then 22
        when 'STN-CABLE-1500' then 9
        when 'STN-FILT-2280' then 1
        when 'STN-SW-5100' then 0
        else i.quantity_on_hand
    end,
    reorder_level = case
        when p.sku in ('STN-COOL-0900', 'STN-FILT-2280', 'STN-BARREL-3000') then 3
        when p.sku in ('STN-COIL-4100', 'STN-COIL-4200') then 2
        else 2
    end,
    updated_at = now()
from parts p
where i.part_id = p.id;

update maintenance_requests set
    title = 'Gun 1 accelerator coil response maintenance',
    description = 'Engineering requested coil and rail service parts after uneven accelerator response during a safe maintenance check.',
    source_panel = 'Barrel Coil Service Panel'
where external_request_id = 'ENG-REQ-2401';

update maintenance_requests set
    title = 'Gun 3 service interlock panel replacement',
    description = 'Approved maintenance request for local interlock panel and pulsed-power isolation fuse replacement.',
    source_panel = 'Control Gallery Panel C'
where external_request_id = 'ENG-REQ-2402';

update maintenance_requests set
    title = 'Gun 5 rail contact lubrication service',
    description = 'Rail contact surfaces require approved lubricant and bore alignment checks before return to engineering.',
    source_panel = 'Rail Service Access'
where external_request_id = 'ENG-REQ-2403';

update maintenance_requests set
    title = 'Gun 2 barrel cooling hose replacement',
    description = 'Cooling manifold hose set requested for preventive maintenance on the barrel cooling loop.',
    source_panel = 'Barrel Cooling Loop'
where external_request_id = 'ENG-REQ-2404';

update maintenance_requests set
    title = 'Gun 7 control trunk cable service',
    description = 'Shielded control trunk cable requested for control-gallery monitor and local panel maintenance.',
    source_panel = 'Control Gallery Panel G'
where external_request_id = 'ENG-REQ-2405';

update maintenance_requests set
    title = 'Gun 4 cradle fastener replenishment',
    description = 'Cradle fastener and shim kit requested for local maintenance shelf replenishment.',
    source_panel = 'Gun Cradle Service Bay'
where external_request_id = 'ENG-REQ-2406';

update maintenance_requests set
    title = 'Gun 8 local panel and sensor harness kit',
    description = 'Local panel module and coil sensor harness requested after engineering diagnostics.',
    source_panel = 'Control Gallery Panel H'
where external_request_id = 'ENG-REQ-2407';

update maintenance_requests set
    title = 'Gun 6 retired cradle switch request',
    description = 'Cancelled engineering request for a retired cradle limit switch.',
    source_panel = 'Cradle Limit Switch Junction'
where external_request_id = 'ENG-REQ-2408';
