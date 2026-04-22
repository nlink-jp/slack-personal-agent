import Foundation
import UserNotifications

// MARK: - Socket protocol

struct Message: Codable {
    let type: String
    let id: String?
    let title: String?
    let subtitle: String?
    let body: String?
    let action: String?
}

// MARK: - Notification delegate

class NotificationDelegate: NSObject, UNUserNotificationCenterDelegate {
    var sendAction: ((String) -> Void)?

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let action = response.notification.request.content.userInfo["action"] as? String ?? ""
        let id = response.notification.request.identifier
        sendAction?(encodeClicked(id: id, action: action))
        completionHandler()
    }

    // Show notifications even when app is in foreground
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([.banner, .sound])
    }

    func encodeClicked(id: String, action: String) -> String {
        let msg = Message(type: "clicked", id: id, title: nil, subtitle: nil, body: nil, action: action)
        let data = try! JSONEncoder().encode(msg)
        return String(data: data, encoding: .utf8)!
    }
}

// MARK: - Socket client

class SocketClient {
    let path: String
    var fileHandle: FileHandle?
    var inputStream: InputStream?

    init(path: String) {
        self.path = path
    }

    func connect() -> Bool {
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { return false }

        var addr = sockaddr_un()
        addr.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutablePointer(to: &addr.sun_path) { ptr in
            path.withCString { cstr in
                _ = memcpy(ptr, cstr, min(path.count, MemoryLayout.size(ofValue: ptr.pointee) - 1))
            }
        }

        let result = withUnsafePointer(to: &addr) { ptr in
            ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockPtr in
                Foundation.connect(fd, sockPtr, socklen_t(MemoryLayout<sockaddr_un>.size))
            }
        }

        guard result == 0 else {
            close(fd)
            return false
        }

        fileHandle = FileHandle(fileDescriptor: fd, closeOnDealloc: true)
        return true
    }

    func readLines(handler: @escaping (String) -> Void) {
        guard let fh = fileHandle else { return }
        DispatchQueue.global().async {
            var buffer = Data()
            while true {
                let data = fh.availableData
                if data.isEmpty { break } // EOF
                buffer.append(data)

                while let range = buffer.range(of: Data([0x0A])) { // newline
                    let line = buffer.subdata(in: buffer.startIndex..<range.lowerBound)
                    buffer.removeSubrange(buffer.startIndex...range.lowerBound)
                    if let str = String(data: line, encoding: .utf8) {
                        handler(str)
                    }
                }
            }
        }
    }

    func writeLine(_ line: String) {
        guard let fh = fileHandle else { return }
        if let data = (line + "\n").data(using: .utf8) {
            fh.write(data)
        }
    }
}

// MARK: - Main

func showNotification(msg: Message) {
    let content = UNMutableNotificationContent()
    content.title = msg.title ?? "slack-personal-agent"
    if let subtitle = msg.subtitle, !subtitle.isEmpty {
        content.subtitle = subtitle
    }
    content.body = msg.body ?? ""
    content.sound = .default
    content.userInfo = ["action": msg.action ?? ""]

    let id = msg.id ?? UUID().uuidString
    let request = UNNotificationRequest(identifier: id, content: content, trigger: nil)

    UNUserNotificationCenter.current().add(request) { error in
        if let error = error {
            fputs("Notification error: \(error)\n", stderr)
        }
    }
}

// Determine socket path
let defaultPath = FileManager.default.homeDirectoryForCurrentUser
    .appendingPathComponent("Library/Application Support/slack-personal-agent/notify.sock")
    .path

let socketPath = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : defaultPath

fputs("spa-notify: connecting to \(socketPath)\n", stderr)

// Request notification permission
let center = UNUserNotificationCenter.current()
let delegate = NotificationDelegate()
center.delegate = delegate

center.requestAuthorization(options: [.alert, .sound]) { granted, error in
    if !granted {
        fputs("spa-notify: notification permission denied\n", stderr)
    }
}

// Connect to socket with retry
let client = SocketClient(path: socketPath)
var connected = false
for attempt in 1...30 {
    if client.connect() {
        connected = true
        fputs("spa-notify: connected\n", stderr)
        break
    }
    fputs("spa-notify: waiting for socket (attempt \(attempt)/30)...\n", stderr)
    Thread.sleep(forTimeInterval: 1.0)
}

guard connected else {
    fputs("spa-notify: failed to connect to \(socketPath)\n", stderr)
    exit(1)
}

// Send click events back to main app
delegate.sendAction = { json in
    client.writeLine(json)
}

// Read notification requests from socket
client.readLines { line in
    guard let data = line.data(using: .utf8),
          let msg = try? JSONDecoder().decode(Message.self, from: data) else {
        return
    }

    if msg.type == "notify" {
        showNotification(msg: msg)
    }
}

// Keep running
RunLoop.main.run()
