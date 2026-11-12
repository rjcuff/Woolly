// The Woolly desktop app: a window and menu bar icon for the local Woolly
// page. It starts `woolly serve` if nothing is running and stops it on quit.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::net::{SocketAddr, TcpStream};
use std::path::PathBuf;
use std::process::{Child, Command};
use std::sync::Mutex;
use std::thread;
use std::time::{Duration, Instant};

use tauri::image::Image;
use tauri::menu::{Menu, MenuItem, PredefinedMenuItem};
use tauri::tray::TrayIconBuilder;
use tauri::{AppHandle, Manager, RunEvent, Url, WebviewUrl, WebviewWindowBuilder, WindowEvent};

const ADDR: &str = "127.0.0.1:11435";

/// The server this app started, if it had to start one.
struct Server(Mutex<Option<Child>>);

fn woolly_home() -> PathBuf {
    if let Ok(h) = std::env::var("WOOLLY_HOME") {
        if !h.trim().is_empty() {
            return PathBuf::from(h);
        }
    }
    let home = std::env::var("HOME").or_else(|_| std::env::var("USERPROFILE")).unwrap_or_default();
    PathBuf::from(home).join(".woolly")
}

fn running() -> bool {
    let addr: SocketAddr = ADDR.parse().expect("valid address");
    TcpStream::connect_timeout(&addr, Duration::from_millis(300)).is_ok()
}

/// The woolly program bundled next to the app.
fn sidecar() -> PathBuf {
    let exe = std::env::current_exe().unwrap_or_default();
    let dir = exe.parent().map(PathBuf::from).unwrap_or_default();
    dir.join(if cfg!(windows) { "woolly.exe" } else { "woolly" })
}

fn start_server() -> Option<Child> {
    if running() {
        return None;
    }
    Command::new(sidecar()).args(["serve", "--proxy"]).spawn().ok()
}

fn token() -> Option<String> {
    std::fs::read_to_string(woolly_home().join("token")).ok().map(|t| t.trim().to_string()).filter(|t| !t.is_empty())
}

/// Waits for the server and its token, up to 15 seconds.
fn wait_ready() -> Option<String> {
    let start = Instant::now();
    while start.elapsed() < Duration::from_secs(15) {
        if running() {
            if let Some(t) = token() {
                return Some(t);
            }
        }
        thread::sleep(Duration::from_millis(200));
    }
    None
}

fn page(token: &str, view: &str) -> Url {
    Url::parse(&format!("http://{ADDR}/auth?token={token}&view={view}")).expect("valid url")
}

fn show(app: &AppHandle, view: Option<&str>) {
    if let Some(w) = app.get_webview_window("main") {
        if let (Some(v), Some(t)) = (view, token()) {
            let _ = w.navigate(page(&t, v));
        }
        let _ = w.show();
        let _ = w.unminimize();
        let _ = w.set_focus();
    }
}

fn main() {
    let app = tauri::Builder::default()
        .manage(Server(Mutex::new(None)))
        .setup(|app| {
            let window = WebviewWindowBuilder::new(app, "main", WebviewUrl::App("index.html".into()))
                .title("Woolly")
                .inner_size(900.0, 760.0)
                .min_inner_size(380.0, 480.0)
                .build()?;

            *app.state::<Server>().0.lock().unwrap() = start_server();
            thread::spawn(move || match wait_ready() {
                Some(t) => {
                    let _ = window.navigate(page(&t, "app"));
                }
                None => {
                    let _ = window.eval(
                        "document.body.innerHTML = '<p>Woolly didn\\'t start. Run <code>woolly serve</code> in a terminal and reopen the app.</p>'",
                    );
                }
            });

            let open = MenuItem::with_id(app, "chat", "Open Woolly", true, None::<&str>)?;
            let memories = MenuItem::with_id(app, "memories", "Manage memories", true, None::<&str>)?;
            let activity = MenuItem::with_id(app, "activity", "Activity", true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "Quit Woolly", true, None::<&str>)?;
            let sep = PredefinedMenuItem::separator(app)?;
            let menu = Menu::with_items(app, &[&open, &memories, &activity, &sep, &quit])?;
            TrayIconBuilder::with_id("woolly")
                .icon(Image::from_bytes(include_bytes!("../icons/tray.png"))?)
                .icon_as_template(true)
                .tooltip("Woolly")
                .menu(&menu)
                .on_menu_event(|app, event| match event.id().as_ref() {
                    "chat" => show(app, Some("app")),
                    "memories" => show(app, Some("current")),
                    "activity" => show(app, Some("activity")),
                    "quit" => app.exit(0),
                    _ => {}
                })
                .build(app)?;
            Ok(())
        })
        .on_window_event(|window, event| {
            // Closing the window keeps Woolly running in the menu bar.
            if let WindowEvent::CloseRequested { api, .. } = event {
                let _ = window.hide();
                api.prevent_close();
            }
        })
        .build(tauri::generate_context!())
        .expect("failed to start Woolly");

    app.run(|app, event| match event {
        #[cfg(target_os = "macos")]
        RunEvent::Reopen { .. } => show(app, None),
        RunEvent::Exit => {
            if let Some(mut child) = app.state::<Server>().0.lock().unwrap().take() {
                let _ = child.kill();
                let _ = child.wait();
            }
        }
        _ => {}
    });
}
