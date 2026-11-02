update shipments
set carrier = 'SCA Motor Pool',
    shipping_method = 'Truck Transport'
where carrier in ('SCA Internal Logistics')
   or shipping_method in ('Ground Courier', 'Priority Courier');
