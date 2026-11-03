# Boilerplate — Teste (JUnit 5)

```java
package {modulo}.{contexto};

import static org.junit.jupiter.api.Assertions.*;

import java.util.Optional;
import org.junit.jupiter.api.Test;

class {Contexto}ServiceTest {
    // gofi:tests REQ-NN.1
    @Test
    void encontra() {
        var service = new {Contexto}Service(id -> Optional.of(new {Entidade}(id)));
        assertEquals(1, service.get(1).id());
    }

    // gofi:tests REQ-NN.2
    @Test
    void recusaQuemNaoExiste() {
        var service = new {Contexto}Service(id -> Optional.empty());
        assertThrows({Entidade}NotFoundException.class, () -> service.get(9));
    }
}
```
