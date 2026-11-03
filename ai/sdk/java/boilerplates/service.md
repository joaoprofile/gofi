# Boilerplate — Service (Quarkus)

```java
package {modulo}.{contexto};

import jakarta.enterprise.context.ApplicationScoped;

@ApplicationScoped
public class {Contexto}Service {
    private final {Contexto}Repository repository;

    public {Contexto}Service({Contexto}Repository repository) {
        this.repository = repository;
    }

    // gofi:implements REQ-NN
    public {Entidade} get(long id) {
        return repository.findById(id).orElseThrow(() -> new {Entidade}NotFoundException(id));
    }
}
```
