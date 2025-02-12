import { z } from "zod";
import { router, publicProcedure } from "../trpc"
import { getProgress, setProgress, updateProgress, resetProgress } from "../services/tutorials_db";

export const tutorialRouter = router({});