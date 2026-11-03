use axum::Router;

// Each context is a module of its own; its routes are merged here.
#[tokio::main]
async fn main() {
    let app = Router::new();
    let port = std::env::var("PORT").unwrap_or_else(|_| "8080".to_string());
    let listener = tokio::net::TcpListener::bind(format!("0.0.0.0:{port}")).await.expect("bind");
    axum::serve(listener, app).await.expect("serve");
}
