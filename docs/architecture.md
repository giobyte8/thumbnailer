# Architecture

## AMQP Messaging

The messaging layer owns RabbitMQ connectivity, queue bindings, publishing, and ack/nack behavior, then hands validated requests to the service layer so business logic stays decoupled from AMQP details.

- `cmd/thumbnailer/main.go` calls `Messaging.Start(ctx)` to initialize AMQP messaging.
- `AMQPResources.ConnectAndSetup()` connects to RabbitMQ, opens separate consumer and producer channels, declares exchange/queues, binds queues, and configures QoS.
- `AMQPConsumer.Start(ctx)` creates one `QueueConsumer` per input queue and starts both concurrently.
- Each `QueueConsumer.Start(...)` reads deliveries from its queue with manual ack/nack behavior.
- `AMQPPublisher` republishes oversized-image requests to the configured alternate queue.
- Message bodies are unmarshaled into `models.ThumbRequest` in consumer callbacks.
- Parsed requests are passed to service entry points:
  - `thumbnailSvc.ProcessGenRequest(ctx, thumbRequest)`
  - `thumbnailSvc.ProcessDelRequest(ctx, thumbRequest)`
- `ThumbnailsService` executes thumbnail business workflows while remaining independent from AMQP transport logic.

```mermaid
flowchart LR
  subgraph RQ[RabbitMQ]
    EX[Direct Exchange]
    QG[Queue: thumbs_gen]
    QD[Queue: thumbs_del]
    EX --> QG
    EX --> QD
  end

  subgraph AC[messaging module]
    S[Messaging.Start]
    CS[AMQPResources.ConnectAndSetup]
    C[AMQPConsumer.Start]
    QCG["QueueConsumer.Start (gen queue)"]
    QCD["QueueConsumer.Start (del queue)"]
    UMG[Unmarshal to ThumbRequest]
    UMD[Unmarshal to ThumbRequest]

    S --> CS --> C
    C --> QCG --> UMG
    C --> QCD --> UMD
  end

  subgraph TS[ThumbnailService module]
    PG[ProcessGenRequest]
    PD[ProcessDelRequest]
  end

  QG --> QCG
  QD --> QCD
  UMG --> PG
  UMD --> PD
```
