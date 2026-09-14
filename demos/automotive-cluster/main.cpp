#include <QApplication>
#include <QDateTime>
#include <QKeyEvent>
#include <QPainter>
#include <QPainterPath>
#include <QTimer>
#include <QWidget>

#include <algorithm>
#include <cmath>

namespace {
class ClusterWindow final : public QWidget {
 public:
  ClusterWindow() {
    setWindowTitle("VAR-Studio Automotive Cluster");
    setMinimumSize(1024, 600);
    setCursor(Qt::BlankCursor);
    auto *timer = new QTimer(this);
    connect(timer, &QTimer::timeout, this, [this]() {
      phase_ += 0.025;
      update();
    });
    timer->start(33);
  }

 protected:
  void paintEvent(QPaintEvent *) override {
    QPainter painter(this);
    painter.setRenderHint(QPainter::Antialiasing);
    QLinearGradient background(0, 0, width(), height());
    background.setColorAt(0, QColor("#071016"));
    background.setColorAt(1, QColor("#101a21"));
    painter.fillRect(rect(), background);
    drawHeader(painter);
    drawGauge(painter);
    drawRoad(painter);
    drawTelemetry(painter);
  }

  void keyPressEvent(QKeyEvent *event) override {
    if (event->key() == Qt::Key_Escape) {
      close();
    }
  }

 private:
  void drawHeader(QPainter &painter) {
    QFont font = painter.font();
    font.setPixelSize(14);
    font.setBold(true);
    painter.setFont(font);
    painter.setPen(QColor("#dbe7eb"));
    painter.drawText(42, 48, "VAR-Studio Digital Cockpit");
    painter.setPen(QColor("#79e8bd"));
    painter.drawText(width() - 250, 48, "●  DRIVE READY");
    painter.setPen(QColor("#78909a"));
    font.setBold(false);
    font.setPixelSize(12);
    painter.setFont(font);
    painter.drawText(
        width() - 250,
        72,
        QDateTime::currentDateTime().toString("ddd  hh:mm"));
  }

  void drawGauge(QPainter &painter) {
    const QPointF center(width() * 0.24, height() * 0.52);
    const qreal radius =
        std::min(width(), height()) * 0.27;
    const int speed = 82 + int(std::sin(phase_) * 8.0);
    const double ratio = speed / 180.0;
    painter.setPen(
        QPen(
            QColor("#23343d"),
            18,
            Qt::SolidLine,
            Qt::RoundCap));
    const QRectF dial(
        center.x() - radius,
        center.y() - radius,
        radius * 2,
        radius * 2);
    painter.drawArc(dial, 220 * 16, -260 * 16);
    painter.setPen(
        QPen(
            QColor("#ff604d"),
            18,
            Qt::SolidLine,
            Qt::RoundCap));
    painter.drawArc(
        dial,
        220 * 16,
        int(-260.0 * ratio * 16.0));
    QFont font = painter.font();
    font.setPixelSize(78);
    font.setBold(true);
    painter.setFont(font);
    painter.setPen(Qt::white);
    painter.drawText(
        QRectF(
            center.x() - radius,
            center.y() - 70,
            radius * 2,
            100),
        Qt::AlignCenter,
        QString::number(speed));
    font.setPixelSize(14);
    font.setLetterSpacing(QFont::AbsoluteSpacing, 3);
    painter.setFont(font);
    painter.setPen(QColor("#78909a"));
    painter.drawText(
        QRectF(
            center.x() - radius,
            center.y() + 32,
            radius * 2,
            30),
        Qt::AlignCenter,
        "KM/H");
  }

  void drawRoad(QPainter &painter) {
    const QRectF area(
        width() * 0.48,
        112,
        width() * 0.30,
        height() - 190);
    QLinearGradient sky(0, area.top(), 0, area.bottom());
    sky.setColorAt(0, QColor("#12272c"));
    sky.setColorAt(1, QColor("#091114"));
    painter.setBrush(sky);
    painter.setPen(QColor("#253b42"));
    painter.drawRoundedRect(area, 18, 18);
    const QPointF horizon(
        area.center().x(),
        area.top() + 115);
    QPainterPath road;
    road.moveTo(horizon.x() - 20, horizon.y());
    road.lineTo(area.left() + 22, area.bottom());
    road.lineTo(area.right() - 22, area.bottom());
    road.lineTo(horizon.x() + 20, horizon.y());
    road.closeSubpath();
    painter.setPen(Qt::NoPen);
    painter.setBrush(QColor("#172227"));
    painter.drawPath(road);
    painter.setPen(QPen(QColor("#6ce4ba"), 3));
    painter.drawLine(
        QPointF(horizon.x() - 20, horizon.y()),
        QPointF(area.left() + 22, area.bottom()));
    painter.drawLine(
        QPointF(horizon.x() + 20, horizon.y()),
        QPointF(area.right() - 22, area.bottom()));
    painter.setPen(
        QPen(QColor("#dbe7eb"), 3, Qt::DashLine));
    const qreal shift =
        std::fmod(phase_ * 70.0, 42.0);
    painter.drawLine(
        QPointF(horizon.x(), horizon.y() + shift),
        QPointF(area.center().x(), area.bottom()));
    painter.setPen(QColor("#dbe7eb"));
    QFont font = painter.font();
    font.setPixelSize(15);
    font.setBold(true);
    painter.setFont(font);
    painter.drawText(
        area.adjusted(24, 20, -24, 0),
        Qt::AlignTop | Qt::AlignLeft,
        "Continue for 2.4 km");
  }

  void infoCard(
      QPainter &painter,
      const QRectF &area,
      const QString &label,
      const QString &value,
      const QString &detail) {
    painter.setPen(QColor("#263943"));
    painter.setBrush(QColor("#101c22"));
    painter.drawRoundedRect(area, 14, 14);
    painter.setPen(QColor("#78909a"));
    painter.drawText(
        area.adjusted(18, 17, 0, 0),
        Qt::AlignLeft | Qt::AlignTop,
        label);
    QFont font = painter.font();
    font.setPixelSize(26);
    font.setBold(true);
    painter.setFont(font);
    painter.setPen(QColor("#eef7f8"));
    painter.drawText(
        area.adjusted(18, 42, 0, 0),
        Qt::AlignLeft | Qt::AlignTop,
        value);
    font.setPixelSize(11);
    font.setBold(false);
    painter.setFont(font);
    painter.setPen(QColor("#79e8bd"));
    painter.drawText(
        area.adjusted(18, 78, 0, 0),
        Qt::AlignLeft | Qt::AlignTop,
        detail);
  }

  void drawTelemetry(QPainter &painter) {
    const qreal left = width() * 0.81;
    const qreal cardWidth = width() - left - 36;
    const qreal cardHeight = 108;
    infoCard(
        painter,
        QRectF(left, 112, cardWidth, cardHeight),
        "BATTERY",
        "78%",
        "312 km range");
    infoCard(
        painter,
        QRectF(left, 232, cardWidth, cardHeight),
        "EFFICIENCY",
        "16.4",
        "kWh / 100 km");
    infoCard(
        painter,
        QRectF(left, 352, cardWidth, cardHeight),
        "DRIVE",
        "D",
        "Regeneration active");
    painter.setPen(QColor("#78909a"));
    const QRect footer(42, height() - 42, width() - 84, 24);
    painter.drawText(
        footer,
        Qt::AlignLeft,
        "DEMO MODE · Simulated vehicle telemetry");
    painter.drawText(
        footer,
        Qt::AlignRight,
        "Press Esc to close");
  }

  double phase_ = 0.0;
};
}  // namespace

int main(int argc, char *argv[]) {
  QApplication app(argc, argv);
  ClusterWindow window;
  window.showFullScreen();
  return app.exec();
}
