"""Flask + SQLAlchemy API blueprint.

Exposes:
    GET  /healthz       -> liveness probe (no DB)
    GET  /items         -> list items
    POST /items         -> create item (json: {"title": str, "body": str?})
    GET  /items/<id>    -> fetch one
    PUT  /items/<id>    -> update
    DEL  /items/<id>    -> delete

DATABASE_URL is required. SQLite works for local dev
(e.g. sqlite:///dev.db); Postgres is the default in .env.example.
"""
from __future__ import annotations

import os
from http import HTTPStatus

from flask import Blueprint, Flask, jsonify, request
from sqlalchemy.exc import SQLAlchemyError

from models import Item, db


def create_app() -> Flask:
    app = Flask(__name__)
    app.config["SQLALCHEMY_DATABASE_URI"] = os.environ.get(
        "DATABASE_URL", "sqlite:///dev.db"
    )
    app.config["SQLALCHEMY_TRACK_MODIFICATIONS"] = False
    db.init_app(app)

    with app.app_context():
        db.create_all()

    app.register_blueprint(health_bp)
    app.register_blueprint(items_bp, url_prefix="/items")

    @app.errorhandler(SQLAlchemyError)
    def handle_db_error(exc: SQLAlchemyError):
        app.logger.exception("database error: %s", exc)
        return jsonify({"error": "database error"}), HTTPStatus.INTERNAL_SERVER_ERROR

    return app


health_bp = Blueprint("health", __name__)


@health_bp.get("/healthz")
def healthz():
    return jsonify({"status": "ok"}), HTTPStatus.OK


items_bp = Blueprint("items", __name__)


@items_bp.get("")
def list_items():
    items = Item.query.order_by(Item.created_at.desc()).all()
    return jsonify([item.to_dict() for item in items])


@items_bp.post("")
def create_item():
    payload = request.get_json(silent=True) or {}
    title = (payload.get("title") or "").strip()
    if not title:
        return jsonify({"error": "title is required"}), HTTPStatus.BAD_REQUEST
    item = Item(title=title, body=payload.get("body"))
    db.session.add(item)
    db.session.commit()
    return jsonify(item.to_dict()), HTTPStatus.CREATED


@items_bp.get("/<int:item_id>")
def get_item(item_id: int):
    item = db.session.get(Item, item_id)
    if not item:
        return jsonify({"error": "not found"}), HTTPStatus.NOT_FOUND
    return jsonify(item.to_dict())


@items_bp.put("/<int:item_id>")
def update_item(item_id: int):
    item = db.session.get(Item, item_id)
    if not item:
        return jsonify({"error": "not found"}), HTTPStatus.NOT_FOUND
    payload = request.get_json(silent=True) or {}
    if "title" in payload:
        title = (payload.get("title") or "").strip()
        if not title:
            return jsonify({"error": "title cannot be empty"}), HTTPStatus.BAD_REQUEST
        item.title = title
    if "body" in payload:
        item.body = payload.get("body")
    db.session.commit()
    return jsonify(item.to_dict())


@items_bp.delete("/<int:item_id>")
def delete_item(item_id: int):
    item = db.session.get(Item, item_id)
    if not item:
        return jsonify({"error": "not found"}), HTTPStatus.NOT_FOUND
    db.session.delete(item)
    db.session.commit()
    return ("", HTTPStatus.NO_CONTENT)


app = create_app()


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=int(os.environ.get("PORT", "8000")))
