# Boilerplate — Resource (Quarkus)

```java
package {modulo}.{contexto};

import jakarta.validation.Valid;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import {modulo}.{contexto}.dto.Create{Entidade}Request;
import {modulo}.{contexto}.dto.{Entidade}Response;

@Path("/{rota}")
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
public class {Contexto}Resource {
    private final {Contexto}Service service;

    public {Contexto}Resource({Contexto}Service service) {
        this.service = service;
    }

    @POST
    public {Entidade}Response create(@Valid Create{Entidade}Request request) {
        return service.create(request);
    }
}
```
