use hyper::service::{make_service_fn, service_fn};
use hyper::{Body, Request, Response, Server};

async fn handle_request(_req: Request<Body>) -> Result<Response<Body>, hyper::Error> {
    // Create a response with a "Hello, World!" message
    let response = Response::builder()
        .header("Content-Type", "text/plain")
        .body(Body::from("Hello, World!\n"))
        .unwrap();

    Ok(response)
}

#[tokio::main]
async fn main() {
    // Create a service to handle incoming requests
    let make_svc = make_service_fn(|_conn| {
        async { Ok::<_, hyper::Error>(service_fn(handle_request)) }
    });

    // Create a server that listens on 127.0.0.1:8080
    let addr = ([127, 0, 0, 1], 8080).into();
    let server = Server::bind(&addr).serve(make_svc);

    println!("Listening on http://{}", addr);

    if let Err(e) = server.await {
        eprintln!("Server error: {}", e);
    }
}
